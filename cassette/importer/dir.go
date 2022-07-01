package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/logutil"
	"github.com/andrebq/boombox/internal/lua/ltoj"
	gluamapper "github.com/yuin/gluamapper"
	lua "github.com/yuin/gopher-lua"
)

type (
	ErrCodebaseNotAllowed struct {
		Base  string
		Asset string
	}

	UserCodeError struct {
		Asset string
		cause error
	}

	auxRoute struct {
		methods []string
		route   string
		asset   string
	}
)

var (
	defaultMapperOptions = gluamapper.Option{
		NameFunc: gluamapper.Id,
		TagName:  "gluamapper",
	}
)

func (e ErrCodebaseNotAllowed) Error() string {
	return fmt.Sprintf("cannot import %v from %v as it represents a code asset", e.Asset, e.Base)
}

func (e UserCodeError) Error() string {
	if e.cause == nil {
		return fmt.Sprintf("user-code generated by file %v", e.Asset)
	}
	return e.cause.Error()
}

func (e UserCodeError) Unwrap() error {
	return e.cause
}

func Directory(ctx context.Context, target *cassette.Control, base string, allowCodebase bool) error {
	var assets []string
	var datasets []string
	base = filepath.Clean(base)
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		// cassettes are restricted to files
		if d.IsDir() {
			return nil
		}
		// all assets must be relative
		assetPath := path[len(base)+1:]
		// datasets undergo a different processing logic
		if strings.HasPrefix(filepath.ToSlash(assetPath), "dataset/") {
			if filepath.Base(path) == "dataset.lua" {
				datasets = append(datasets, assetPath)
				// dataset.lua is considered a public asset as it describes
				// the relational data available in the cassette.
				//
				// The content of the files will be available as relational tables
				// within the cassette
				assets = append(assets, assetPath)
			}
			return nil
		}
		assets = append(assets, assetPath)
		return nil
	})
	var routes []auxRoute
	for _, f := range assets {
		if filepath.Base(f) == "routes.lua" {
			err := scanRoutes(ctx, &routes, filepath.Join(base, f))
			if err != nil {
				return err
			}
			continue
		}
		codebase, asset, err := ImportFile(ctx, target, base, f, allowCodebase)
		if err != nil {
			return err
		}
		if codebase {
			err = target.ToggleCodebase(ctx, asset, true)
			if err != nil {
				return err
			}
		}
	}
	for _, r := range routes {
		err := target.MapRoute(ctx, r.methods, r.route, r.asset)
		if err != nil {
			return err
		}
	}
	for _, d := range datasets {
		err := importDataset(ctx, target, base, d)
		if err != nil {
			return err
		}
	}
	return nil
}

func importDataset(ctx context.Context, target *cassette.Control, base string, dataset string) error {
	log := logutil.GetOrDefault(ctx).With().Str("dirname", base).Str("asset", dataset).Logger()
	l := lua.NewState(lua.Options{SkipOpenLibs: true})
	datasources := map[string]map[string]interface{}{}
	datasetDir := filepath.Dir(filepath.Join(base, dataset))
	tableAssetDir := path.Dir(filepath.ToSlash(dataset))
	l.SetField(l.G.Global, "add_datasource", l.NewFunction(func(l *lua.LState) int {
		srcFile := path.Clean(l.CheckString(1))
		log = log.With().Str("srcFile", srcFile).Logger()
		fullpath := filepath.Join(datasetDir, filepath.FromSlash(srcFile))
		if stat, err := os.Lstat(fullpath); err != nil {
			log.Error().Err(err).Msg("Unable to inspect datasource file")
			l.RaiseError("unable to load datasource: %v", l.CheckString(1))
		} else if stat.IsDir() {
			l.RaiseError("unable to load datasource: %v, cannot process directories", l.CheckString(1))
		}
		descriptor := ltoj.ToJSONValue(l.CheckTable(2)).(map[string]interface{})
		descriptor["importedFromFile"] = srcFile
		datasources[srcFile] = descriptor
		l.Push(lua.LTrue)
		return 1
	}))
	l.SetField(l.G.Global, "load_csv", l.NewFunction(func(l *lua.LState) int {
		srcFile := path.Clean(l.CheckString(1))
		log = log.With().Str("srcFile", srcFile).Logger()
		fullpath := filepath.Join(datasetDir, filepath.FromSlash(srcFile))
		if stat, err := os.Lstat(fullpath); err != nil {
			log.Error().Err(err).Msg("Unable to inspect datasource file")
			l.RaiseError("unable to load datasource: %v", l.CheckString(1))
		} else if stat.IsDir() {
			l.RaiseError("unable to load datasource: %v, cannot process directories", l.CheckString(1))
		}
		tableName := l.CheckString(2)
		reader, err := os.Open(fullpath)
		if err != nil {
			log.Error().Err(err).Msg("unable to open datasource file")
			l.RaiseError("unable to load datasource: %v, file could not be opened for read", l.CheckString(1))
		}
		defer reader.Close()
		rows, err := target.ImportCSVDataset(ctx, tableName, reader)
		if err != nil {
			log.Error().Err(err).Msg("unable to import CSV into cassete")
			l.RaiseError("unable to load datasource: %v, cassette.ImportCSVDataset failed", l.CheckString(1))
		}

		tableDDL, err := target.TableDDL(ctx, tableName)

		descriptor := datasources[srcFile]
		descriptor["ddl"] = map[string]string{
			"create": tableDDL,
		}
		buf, err := json.Marshal(descriptor)
		if err != nil {
			log.Error().Err(err).Msg("uanble to convert datasource config to JSON")
			l.RaiseError("unable to load datasource: %v, error encoding as JSON", l.CheckString(1))
		}

		// now that we loaded the table
		// let's map it to an asset path, so discovery is easier
		_, err = target.StoreAsset(ctx, path.Join(tableAssetDir, fmt.Sprintf("%v.json", tableName)), "application/json", string(buf))
		if err != nil {
			log.Error().Err(err).Msg("Unable to import CSV into casset")
			l.RaiseError("unable to load datasource: %v, could not store table descriptor as asset", srcFile)
		}
		l.Push(lua.LNumber(float64(rows)))
		return 1
	}))

	code, err := ioutil.ReadFile(filepath.Join(base, dataset))
	if err != nil {
		return err
	}
	err = l.DoString(string(code))
	if err != nil {
		return err
	}
	return nil
}

func scanRoutes(ctx context.Context, out *[]auxRoute, ap string) error {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	L.SetField(L.G.Global, "add_route", L.NewFunction(lua.LGFunction(func(L *lua.LState) int {
		route := L.CheckString(1)
		method := L.CheckString(2)
		asset := L.CheckString(3)
		*out = append(*out, auxRoute{
			route:   route,
			methods: strings.Split(strings.ToUpper(method), "|"),
			asset:   asset,
		})
		return 0
	})))
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	L.SetContext(ctx)
	content, err := ioutil.ReadFile(ap)
	if err != nil {
		return err
	}
	err = L.DoString(string(content))
	if err != nil {
		return UserCodeError{Asset: ap, cause: err}
	}
	return nil
}

func MimetypeFromExtension(ext string) string {
	var mt string
	switch ext {
	// TODO: implement a better mime-type discovery logic
	case ".lua":
		mt = "text/x-lua"
	case ".js":
		mt = "application/javascript"
	case ".json":
		mt = "application/json"
	case ".html":
		mt = "text/html"
	case ".css":
		mt = "text/css"
	default:
		mt = "application/octet-stream"
	}
	return mt
}

func ImportFile(ctx context.Context, target *cassette.Control, base string, asset string, allowCodebase bool) (bool, string, error) {
	assetPath := filepath.ToSlash(asset)
	if !allowCodebase && strings.HasPrefix(assetPath, "codebase/") {
		return false, "", ErrCodebaseNotAllowed{Base: base, Asset: asset}
	}
	mt := MimetypeFromExtension(filepath.Ext(asset))
	content, err := ioutil.ReadFile(filepath.Join(base, asset))
	if err != nil {
		return false, "", err
	}
	_, err = target.StoreAsset(ctx, assetPath, mt, string(content))
	codebase := strings.HasPrefix(asset, "codebase/") && mt == "text/x-lua"
	return codebase, asset, err
}
