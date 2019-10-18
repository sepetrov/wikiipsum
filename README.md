# wikiipsum

[Lorem Ipsum](https://en.wikipedia.org/wiki/Lorem_ipsum) generator using content from [Wikipedia](https://wikipedia.org).

---

A small program written in Go to generate random text in specified language using the [Wikimedia REST API](https://www.mediawiki.org/wiki/REST_API).

## Compilation

You need [Go](https://golang.org). Download and install it if you don't have it yet.

Clone the repository.
```bash
git clone git@github.com:sepetrov/wikiipsum.git .
```

Navigate to the directory with the source code.
```bash
cd wikiipsum
```

Compile a binary.
```bash
go build -o wikiipsum .
```

## Download

To download a precompiled binary visit the [releases page](https://github.com/sepetrov/wikiipsum/releases).

## Usage

```bash
Lorem Ipsum generates text using content from Wikipedia and prints it to the standard output.
For more information visit https://github.com/sepetrov/wikiipsum.

Example:

	./wikiipsum -user-agent="admin@example.com" -lang="en" -length="500"

Usage of ./wikiipsum:
  -lang string
    	Language code, e.g. 'en'
  -length string
    	Length of generated text, e.g. '500' for 500 bytes, '100 bytes', '100 Kb', '1.5 MB' etc.
  -rate float
    	Request rate limit in req/s
  -user-agent string
    	User agent header for API calls to Wikipedia. It should provide information how to contact you, e.g. admin@example.com
  -verbose
    	Verbose
```

## License

See [LICENSE](LICENSE).