# Preparing the original game data

R-Type's art, maps, fonts and audio are generated locally from the user's Amiga
ADF. The repository does not include the disk, extracted executable ranges or
generated runtime directory.

## Supported disk

The verified 901,120-byte image has SHA-256:

```text
5a481e82ce563d0b61faf96d5a41fd2762be79f2336ec3edbf1e704f30fe763d
```

It is commonly named:

```text
R-Type (1989)(Electric Dreams).adf
```

The [Planet Emulation Amiga ADF catalogue, letter R](https://www.planetemu.net/roms/commodore-amiga-games-adf?page=R)
can be used to identify the release. Extract the ADF from its archive first.

## Generate and run

```sh
go run ./cmd/extract \
  -adf "/path/to/R-Type (1989)(Electric Dreams).adf" \
  -out assets/runtime
go run ./cmd/rtype
```

The extractor validates the disk before generating the eight maps, tile banks,
sprites, interface art, fonts, TFMX music, effects and manifest. Unknown disk
revisions are rejected.

Desktop and Android builds must use the same generated `assets/runtime`
directory. Keep the ADF and generated assets local and list them in
`.git/info/exclude`; the repository intentionally has no `.gitignore`.
