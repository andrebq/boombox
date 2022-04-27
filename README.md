# boombox - Share data with anyone anywhere anytime

This project is definetly inpsired by https://datasette.io/,
and I was interested in exploring that idea.

It also serves as a way to expriment with the concept of sharing
data and allowing arbitrary _*_ user-code in a server. The knowledge
acquired here will be used in other project that I am working at the
moment.

With that said, I expect this to be interesting but not particuarly
production ready (aka don't complain if lose data of someone hacks
your server)


## Main concepts

The name comes from the old [boombox](https://en.wikipedia.org/wiki/Boombox),
kids these days would use a bluetooth speaker with a builtin battery. I'm
old soo are my references :-). Deal with it!

Each active user session has a **control cassette** associated with it,
that cassette contains:

- Web assets (html pages, javascript, css, etc...)
- Dynamic page blocks (currently only lua scripts are supported)
  - Pages might return JSON objects as well
- Routing that maps a given URL (or collection of URLS) to a given dynamic page

Then, users can explorer one or more **data cassettes**, those have less capabilities
than a **control cassette**, they can:

- Contain one or more tables with data that can be queried by the user (arbitrary
  queries are allowed).
- Routing that maps URLs to dynamic code blocks
  - These code blocks can only return JSON encoded data and are restricted to
    reading data from the **data cassette** they are stored.

### What is a cassette

A cassette is just a sqlite database. **control cassettes** can be written by
users (assuming you implement proper authorization management). **Data cassettes**
on the other hand are read-only. To update them a user has to upload an entire new
**cassette** (again assuming proper authorization management).
