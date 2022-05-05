add_datasource('germany-wind-energy.csv', {
  url='https://www.kaggle.com/datasets/aymanlafaz/wind-energy-germany',
  license='CC0: Public Domain',
  originalSource='https://open-power-system-data.org/'
})

load_csv('germany-wind-energy.csv', 'wind')
