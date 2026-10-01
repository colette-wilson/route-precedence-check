# routecheck

A routing table that grows for a year ends up with patterns that overlap in
ways nobody planned. `/users/{id}` and `/users/me` both look reasonable on
their own; put them in the same table and whichever one your router happens
to prefer determines whether "me" resolves as a literal page or gets parsed
as a user id. Most routers pick a winner silently. This tool answers the one
question you actually have when that goes wrong: for this exact request,
which pattern wins, and what else in the table would also have matched?

It reads a plain text file of route patterns and tells you, for a given
method and path, which one is chosen and why it beat the others.

## Pattern syntax

One pattern per line. Blank lines and lines starting with `#` are ignored.

```
GET /users/{id}
GET /users/me
GET /files/{path...}
POST /users/
/health
```

- An optional leading method (`GET`, `POST`, etc.). No method means "matches
  any method".
- `{name}` matches exactly one path segment and captures it.
- `{name...}` must be the last segment; it captures everything remaining,
  including slashes.
- A pattern ending in `/` also matches anything below it (a subtree match),
  the same convention `net/http`'s `ServeMux` uses.
- `{$}` as the last segment matches the path exactly and turns off the
  subtree behavior. `GET /images/` matches `/images/` and everything under
  it; `GET /images/{$}` matches only `/images/` itself. Use it when a
  subtree pattern and an exact one need to coexist in the same table.

- An optional host goes right before the path: `GET example.com/docs/`.
  Host patterns only match requests made with `-host example.com` (case
  and port are ignored), and they beat any pattern without a host.

## Usage

```
$ go build -o routecheck .
$ ./routecheck [-host name] routes.txt GET /users/me

match: GET /users/me (line 2)

1 other pattern(s) also match this request:
  GET /users/{id} (line 1)
```

The second line tells you the thing a router won't: `/users/{id}` was also a
candidate. If your framework's precedence rules ever change, or you port the
table to a router with different rules, that's the pattern that will start
stealing traffic from `/users/me`.

A request with no ambiguity just reports the single winner:

```
$ ./routecheck routes.txt GET /files/reports/2024/q1.pdf

match: GET /files/{path...} (line 3)
  path = reports/2024/q1.pdf
```

## How the winner is picked

Routers disagree on precedence, so this tool doesn't try to imitate any one
of them exactly. It uses the same intuition Go 1.22's `http.ServeMux` is
built on: literal segments beat wildcard segments, a pattern with an
explicit method beats one without, and a subtree match (trailing `/`) is
the least specific option of all. That covers the overlaps you're most
likely to hit in practice; it is not a guarantee that it matches your
framework's own tie-breaking rules exactly.

## Status

Early. A stricter reimplementation of `ServeMux`'s actual precedence
algorithm is not in yet.
