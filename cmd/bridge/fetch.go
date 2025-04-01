package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/basemachina/bridge"
	"github.com/basemachina/bridge/internal/auth"
	"github.com/go-logr/logr"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

var _ auth.PublicKeyGetter = (*FetchWorker)(nil)

type FetchWorker struct {
	sync.RWMutex

	// config
	apiURL   *url.URL
	interval time.Duration
	timeout  time.Duration

	// store
	publicKey jwk.Set
	readyOnce sync.Once

	// Once a public-key is obtained, it becomes ready.
	readyCh chan struct{}
	// Emits if it encounters an error that cannot be made ready.
	readyErrCh chan error

	// canceller
	ctx context.Context

	logger logr.Logger
}

// NewFetchWorker creates a new worker to fetch (or update) public-key.
func NewFetchWorker(env *bridge.Env, l logr.Logger) (*FetchWorker, func(), error) {
	u, err := url.Parse(env.APIURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse %q: %w", env.APIURL, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &FetchWorker{
		apiURL:     u,
		interval:   env.FetchInterval,
		timeout:    env.FetchTimeout,
		ctx:        ctx,
		readyCh:    make(chan struct{}),
		readyErrCh: make(chan error, 1),
		logger:     l,
	}
	return f, cancel, nil
}

// - If the public key has never been obtained
//   - If status code is kind of 400, I want the process to die.
//   - If the error is retriable, keep retrying until it can be obtained
//   - Wait for serve until public key can be obtained
//
// - If you have already obtained the public key
//   - If it is a retriable error, keep retrying until it can be obtained.
//   - Even if status code is kind of 400, the process continues to use the previously
//     acquired public key without dying, and retries to acquire a new key.
func (f *FetchWorker) StartWorker() {
	go func() {
		ctrler := newWorkController(f.interval)
		defer ctrler.Stop()
		defer f.logger.Info("finished running worker")

		for ctrler.Next(f.ctx) {
			publicKey, err := f.fetchPublicKey()
			if err != nil {
				if errors.Is(err, ErrRetryable) {
					ctrler.Retry()
					time.Sleep(3 * time.Second)
					continue
				}

				select {
				// If you have already obtained the public-key
				// Wait for the next ticker to emit
				case <-f.readyCh:
					f.logger.Error(err, "failed to refresh public-key",
						"retry after", f.interval,
					)
					continue
				default:
				}

				// If the public-key has never been obtained, the
				// return error and immediately finish the process
				f.readyErrCh <- err
				return
			}
			f.RWMutex.Lock()
			f.publicKey = publicKey
			f.readyOnce.Do(func() { close(f.readyCh) })
			f.RWMutex.Unlock()
		}
	}()
}

func (f *FetchWorker) WaitForReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.ctx.Done():
		return f.ctx.Err()
	case err := <-f.readyErrCh:
		return err
	case <-f.readyCh:
		return nil
	}
}

type workController struct {
	retryCh chan struct{}
	ticker  *time.Ticker
}

func newWorkController(interval time.Duration) *workController {
	retryCh := make(chan struct{}, 1)
	retryCh <- struct{}{} // to invoke immediately
	return &workController{
		retryCh: retryCh,
		ticker:  time.NewTicker(interval),
	}
}

func (w *workController) Retry() {
	w.retryCh <- struct{}{}
}

func (w *workController) Stop() {
	w.ticker.Stop()
}

func (w *workController) Next(ctx context.Context) bool {
	// In select, there is no order guarantee for channel processing.
	// If the context is cancelled and this method is called again
	// Always return false when called.
	select {
	case <-ctx.Done():
		return false
	default:
	}

	// no order guarantee, but while retryCh, ticker's channel is not sent
	// returns false if context is cancelled
	select {
	case <-ctx.Done():
		return false
	case <-w.retryCh:
	case <-w.ticker.C:
	}
	return true
}

func (f *FetchWorker) GetPublicKey() jwk.Set {
	f.RWMutex.RLock()
	publicKey := f.publicKey
	f.RWMutex.RUnlock()
	return publicKey
}

const publicKeyTargetPath = "/v1/bridge_authn_pubkey"

var ErrRetryable = errors.New("retry")

func (f *FetchWorker) fetchPublicKey() (jwk.Set, error) {
	url := f.buildURL(publicKeyTargetPath)
	ctx, cancel := context.WithTimeout(f.ctx, f.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new request: %w", err)
	}
	req.Header.Add("User-Agent", serviceName+"/"+version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done(): // check timeout or not
			// retry
			return nil, ErrRetryable
		default:
		}
		return nil, fmt.Errorf("failed to send request %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// retry
		if 500 <= resp.StatusCode && resp.StatusCode <= 599 {
			return nil, ErrRetryable
		}
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	key, err := jwk.ParseReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}
	return key, nil
}

func (f *FetchWorker) buildURL(endpoint string) string {
	u := cloneURL(f.apiURL)
	u.Path = path.Join(u.Path, endpoint)
	return u.String()
}

func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	u2 := new(url.URL)
	*u2 = *u
	// if u.User != nil {
	// 	u2.User = new(url.Userinfo)
	// 	*u2.User = *u.User
	// }
	return u2
}
