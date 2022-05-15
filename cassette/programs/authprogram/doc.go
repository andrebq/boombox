// Package authprogram modifies a cassette to include authentication tables
// that can be used to authenticate http requests.
//
// The actual auth logic is not kept in the cassette itself because the
// access control should happen before the cassette is accessed.
//
// To avoid any stupid mistake on my part, I've decided to make it simpler
// for now.
//
// What makes this "special" is that I'm making the life of adversaries
// much easier by letting them download the entire authentication table.
//
// Yeah, your read it right, this auth system assumes everyone on the internet
// is able to read all user_ids, salted passwords, etc...,
// and yet, an adversary should have a hard time trying to recover
// any user data.
//
// Ofcourse this means an adversary with enough computation power to break
// NaCL encryption and reverse an HMAC hash, could recover usernames.
//
// Even if they get that, an adversary will not be able to recover the
// password because the password is never kept in the database.
//
// The password authentication scheme will take a password provided by
// the user, extended it using Argon2id to a 32 byte key,
// which will be used to encrypt a known plain-text. Even if an adversary,
// is able to recover the 32 byte key (quite unlikely), it will not make it
// easier to guess the original password.
//
// Then, that hash will be compared to what is in the database,
// if they match, a new random token is given to the user (either as a cookie)
// or as a string.
//
// Then that token will be kept in memory and future access can be validated
// against that cookie instead of running the whole auth scheme again.
//
// If the token is lost the user should redo the login to obtain a new one,
// tokens might be lost if they expire, service is restarted, or the entry
// is evicted from cache
package authprogram
