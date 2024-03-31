# Palace

This is a simple search engine, but only for websites I've already seen. It
consists of a web extension that uploads the `.innerText` to a simple server and
SQLite database using the [FTS5 extension](https://www.sqlite.org/fts5.html).

It acts like a sort of mind palace. If I've seen something once, I can close the
tab and trust I can search for its text later, even if it's something that would
be hard (or impossible) to find on Google.

Ironically, I am certain I have seen a similar project before, but I can no
longer find it.

## Using it

You can run the server locally after setting some environment variables:

- `AUTH_SALT` - A salt to be used for hashing passwords.
- `AUTH_BLOCK_KEY` - An AES encryption key (for secure login cookies).
- `AUTH_HASH_KEY` - A key for HMAC (for secure login cookies).
- `MY_PASSWORD` - Your salted and hashed password (same salt as before).
- `DB_FILE` - The file to store data and index in.
- `PATH_PREFIX` - Should be left unset unless running behind a path prefix
  proxy.

The extension can be loaded from the directory extension/ using a browser in
developer mode.

## Not using it

It's probably not a good idea to keep a database with the contents of every
website you visit. The concept is a security risk.

I've written this to be deliberately inflexible and single-user. There is no
registration path and my own username is hardcoded.
