package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"mime"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"golang.org/x/time/rate"
)

var usage = `Lorem Ipsum generates text using content from Wikipedia and prints it to the standard output.
For more information visit https://github.com/sepetrov/wikiipsum.

Example:

	%s -user-agent="admin@example.com" -lang="en" -length="500"

Usage of %s:
`

// Version is the program version.
// See https://github.com/golang/go/wiki/GcToolchainTricks#including-build-information-in-the-executable.
var Version string

// See https://en.wikipedia.org/api/rest_v1/.
func main() {
	var (
		userAgent string
		lang      string
		lengthStr string
		rateLimit float64
		verbose   bool
		version   bool
	)

	flag.StringVar(&userAgent, "user-agent", "", "User agent header for API calls to Wikipedia. It should provide information how to contact you, e.g. admin@example.com")
	flag.StringVar(&lang, "lang", "", "Language code, e.g. 'en'")
	flag.StringVar(&lengthStr, "length", "", "Length of generated text, e.g. '500' for 500 bytes, '100 bytes', '100 Kb', '1.5 MB' etc.")
	flag.Float64Var(&rateLimit, "rate", 0, "Request rate limit in req/s")
	flag.BoolVar(&verbose, "verbose", false, "Verbose")
	flag.BoolVar(&version, "version", false, "Print version")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), usage, os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if version {
		fmt.Fprintln(os.Stdout, Version)
		os.Exit(0)
	}

	if userAgent == "" {
		fmt.Println("'-user-agent' is required")
		os.Exit(1)
	}
	if lang == "" {
		fmt.Println("'-lang' is required")
		os.Exit(1)
	}
	length, err := str2bytes(lengthStr)
	if err != nil {
		fmt.Println("'-length' is invalid: " + err.Error())
		os.Exit(1)
	}
	if rateLimit <= 0 || rateLimit > maxRateLimit {
		rateLimit = maxRateLimit
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Rate limit: %f\n", rateLimit)
	}

	wiki := wikiClient{
		url:       fmt.Sprintf(randomSummaryURL, lang),
		userAgent: userAgent,
		client:    http.Client{Timeout: 5 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	txtch := make(chan []byte) // Channel to send random text.
	errch := make(chan error)  // Channel to send errors.

	go func() {
		sleepch := make(chan time.Duration) // Channel to pause creating go routines.
		lim := rate.NewLimiter(rate.Limit(rateLimit), 1)
		for {

			// Pause when we need to back off due to errors.
			select {
			case d := <-sleepch:
				time.Sleep(d)
			default:
			}

			// Wait for the next available event so we don't exceed the rate limit.
			if err := lim.Wait(ctx); err != nil {
				errch <- err
				return
			}

			go func() {
				op := func() error {
					if verbose {
						fmt.Fprint(os.Stderr, ".")
					}
					b, err := wiki.RandomSummary(ctx)
					if errors.Is(err, errTooManyRequests) {
						errch <- err
						return err // Back off when we have 429 Too Many Requests response.
					}
					if err != nil {
						errch <- err
						return nil
					}
					txtch <- b
					return nil
				}
				notify := func(_ error, next time.Duration) {
					sleepch <- next
				}

				if err := backoff.RetryNotify(op, backoff.NewExponentialBackOff(), notify); err != nil {
					errch <- err
				}
			}()
		}
	}()

	sigch := make(chan os.Signal) // Channel to send OS signal to terminate this program.
	l := 0
	for {
		select {
		case txt := <-txtch:
			n, _ := fmt.Fprintln(os.Stdout, string(txt))
			if length > 0 {
				l += n
				if l >= length {
					goto End
				}
			}
		case err := <-errch:
			var ignore bool
			var timeoutErr interface {
				Timeout() bool
			}
			if errors.Is(err, errTooManyRequests) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) ||
				errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
				ignore = true
			}
			if verbose || !ignore {
				fmt.Fprintln(os.Stderr, err)
			}
		case sig := <-sigch:
			if sig == os.Interrupt || sig == os.Kill {
				goto End
			}
		}
	}

End:
	cancel()
}

// byteMap contains a map of regular expressions for conversion of strings to bytes.
var byteMap = map[*regexp.Regexp]float64{
	regexp.MustCompile(`^(\d+)$`):                1,
	regexp.MustCompile(`^(\d+)\s?(byte)$`):       1,
	regexp.MustCompile(`^(\d+)\s?(byte)$`):       1,
	regexp.MustCompile(`^(\d+)\s?(bytes)$`):      1,
	regexp.MustCompile(`^(\d+(\.\d+)?)\s?(Kb)$`): 1024,
	regexp.MustCompile(`^(\d+(\.\d+)?)\s?(MB)$`): 1024 * 1024,
}

// str2bytes parses s and tries to returns the corresponding size in bytes.
func str2bytes(s string) (int, error) {
	for r, f := range byteMap {
		match := r.FindStringSubmatch(s)
		if len(match) == 0 {
			continue
		}
		val, err := strconv.ParseFloat(match[1], 10)
		if err != nil {
			return 0, err
		}
		return int(math.Round(val * f)), nil
	}

	return 0, errors.New("cannot parse string")
}

// randomSummaryURL is the fmt.Sprintf pattern of the Wikipedia API URL
// for random page summary. Use two character language code as an argument.
//
//	fmt.Sprintf(randomSummaryURL, "en")
const randomSummaryURL = "https://%s.wikipedia.org/api/rest_v1/page/random/summary"

// maxRateLimit is the maximum rate limit for API calls to Wikipedia.
// See https://en.wikipedia.org/api/rest_v1/.
const maxRateLimit float64 = 200

type wikiClient struct {
	url       string
	userAgent string
	client    http.Client
}

var errTooManyRequests = errors.New("too many requests")

// RandomText returns text from a random Wikipedia page.
func (w *wikiClient) RandomSummary(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/problem+json")
	req.Header.Add("User-Agent", w.userAgent)

	resp, err := w.client.Do(req)

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	var timeoutErr interface {
		Timeout() bool
	}
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, errTooManyRequests
	}

	fail := func(format string, a ...interface{}) ([]byte, error) {
		a = append(a, req, resp)
		return nil, fmt.Errorf(format+"\n\nrequest:\n%v\n\nresponse:\n%v\n", a...)
	}

	if resp.StatusCode != http.StatusOK {
		return fail("response status %s", resp.Status)
	}

	if ctype, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err != nil {
		return nil, err
	} else if ctype != "application/json" {
		return fail("response content type %q", ctype)
	}

	var body struct{ Extract string `json:"extract"` }
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fail("%w", err)
	}

	return []byte(strings.TrimSpace(body.Extract)), nil
}
