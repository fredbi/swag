* small corrections to the ucd set: greek lambda etc
* [x] expose numbers.Romans(int64)
* [x]  fix ToASCII / ASCIIRune for non-ascii digits
* [x] ensure complete ascii_fold (diacritics - generated)
* [x] ensure Jamo rune names are correctly derived from base unicode extraction ~/ otherwise supplement with Jamo.txt~ : we skip it
* [x] docstrings: move Mangler detailed notes to package-level - see if links to this work in pkgsite
* prepare unicode v17 with build guards (see /home/fred/src/golang.org/x/text/unicode/runenames/tables17.0.0.go)

