* small corrections to the ucd set: greek lambda etc
* [x] expose numbers.Romans(int64)
* [x]  fix ToASCII / ASCIIRune for non-ascii digits
* [x] ensure complete ascii_fold (diacritics - generated)
* [x] ensure Jamo rune names are correctly derived from base unicode extraction ~/ otherwise supplement with Jamo.txt~ : we skip it
* [x] docstrings: move Mangler detailed notes to package-level - see if links to this work in pkgsite
* [x] prepare unicode v17 with build guards (dual tables `…15.0.0.go` `!go1.27` / `…17.0.0.go` `go1.27`; version registry in ucd/internal/locate; green under go1.26 + go1.27rc1)
* make gen_runewords toolchain-independent: classify from a UCD category extract instead of unicode.Is (so `go generate` under any Go produces the correct v17 table), or warn when runtime.Version() < the target's MinGo

