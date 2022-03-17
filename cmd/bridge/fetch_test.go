package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/basemachina/bridge/internal/auth"
)

func ServeJWK(publicJWK []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Write(publicJWK)
	}
}

func Test_newWorkController(t *testing.T) {
	ctrler := newWorkController(time.Second)
	if ctrler.ticker == nil {
		t.Error("want non nil ticker")
	}
	select {
	case <-ctrler.retryCh:
	case <-time.After(time.Second):
		t.Error("want retry channel")
	}
}

func Test_newWorkController_Next(t *testing.T) {
	interval := 300 * time.Millisecond
	ctrler := newWorkController(interval)
	ctx := context.Background()

	// check initialize
	if !ctrler.Next(ctx) || len(ctrler.retryCh) != 0 {
		t.Errorf("want Next is true and removed channel from retryCh: %d", len(ctrler.retryCh))
	}

	ctrler.Retry()
	if !ctrler.Next(ctx) || len(ctrler.retryCh) != 0 {
		t.Errorf("(retry) want Next is true and removed channel from retryCh: %d", len(ctrler.retryCh))
	}

	cctx, cancel := context.WithTimeout(ctx, interval)
	if !ctrler.Next(cctx) {
		t.Errorf("want Next is true and use ticker")
	}

	cancel()
	if ctrler.Next(cctx) {
		t.Error("want Next is false")
	}

	// 必ず false になってるか確認
	// もし実装がおかしい場合、テストがランダムにこける可能性がある
	ctrler.Retry()
	if ctrler.Next(cctx) {
		t.Error("want Next is false (with retryCh)")
	}
}

func TestFetchWorker_fetchPublicKey(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		_, pubKey, err := auth.GetJWKKeys()
		if err != nil {
			t.Fatal(err)
		}

		publicJWK, err := json.Marshal(pubKey)
		if err != nil {
			t.Fatal(err)
		}

		mux := http.NewServeMux()
		mux.Handle(publicKeyTargetPath, ServeJWK(publicJWK))
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatal(err)
		}

		f := &FetchWorker{
			apiURL:  u,
			ctx:     context.Background(),
			timeout: time.Second,
		}

		set, err := f.fetchPublicKey()
		if err != nil {
			t.Fatal(err)
		}
		if set == nil {
			t.Fatalf("want public key set")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		invalidCases := []struct {
			name              string
			timeout           time.Duration
			handler           http.HandlerFunc
			wantUnderlying    bool
			wantUnderlyingErr error
		}{
			{
				name:              "timeout",
				timeout:           0,
				handler:           func(http.ResponseWriter, *http.Request) {},
				wantUnderlying:    true,
				wantUnderlyingErr: ErrRetryable,
			},
			{
				name:    "response code 500",
				timeout: time.Second,
				handler: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				},
				wantUnderlying:    true,
				wantUnderlyingErr: ErrRetryable,
			},
			{
				name:    "response code 400",
				timeout: time.Second,
				handler: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
				},
				wantUnderlying: false,
			},
		}
		for _, tc := range invalidCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				mux := http.NewServeMux()
				mux.Handle(publicKeyTargetPath, tc.handler)
				srv := httptest.NewServer(mux)
				t.Cleanup(srv.Close)
				u, err := url.Parse(srv.URL)
				if err != nil {
					t.Fatal(err)
				}

				fw := &FetchWorker{
					apiURL:  u,
					ctx:     context.Background(),
					timeout: tc.timeout,
				}
				_, err = fw.fetchPublicKey()
				if err == nil {
					t.Fatal("want error")
				}
				if tc.wantUnderlying != errors.Is(err, tc.wantUnderlyingErr) {
					t.Fatalf("want error %v, but got error %v", tc.wantUnderlyingErr, err)
				}
			})
		}

		t.Run("worker is canncelled", func(t *testing.T) {
			mux := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			u, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatal(err)
			}

			// make cancelled context
			ctx := context.Background()
			cctx, cancel := context.WithCancel(ctx)
			cancel()

			fw := &FetchWorker{
				apiURL:  u,
				ctx:     cctx,
				timeout: time.Second,
			}
			_, err = fw.fetchPublicKey()
			if err == nil {
				t.Fatal("want error")
			}
			if !errors.Is(err, ErrRetryable) {
				t.Errorf("want underlying error %q, but got %q", context.Canceled, err)
			}
		})
	})
}

func TestFetchWorker_WaitForReady(t *testing.T) {
	cases := []struct {
		name       string
		readyCh    chan struct{}
		readyErrCh chan error
		ctx        context.Context
		fctx       context.Context // for field
		wantErr    bool
	}{
		{
			name: "ready",
			readyCh: func() chan struct{} {
				ch := make(chan struct{})
				close(ch)
				return ch
			}(),
			ctx:     context.Background(),
			fctx:    context.Background(),
			wantErr: false,
		},
		{
			name: "ready error",
			readyErrCh: func() chan error {
				ch := make(chan error, 1)
				ch <- fmt.Errorf("error")
				return ch
			}(),
			ctx:     context.Background(),
			fctx:    context.Background(),
			wantErr: true,
		},
		{
			name:    "cancelled context",
			readyCh: make(chan struct{}),
			ctx: func() context.Context {
				ctx := context.Background()
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				return cctx
			}(),
			fctx:    context.Background(),
			wantErr: true,
		},
		{
			name:    "cancelled field context",
			readyCh: make(chan struct{}),
			ctx:     context.Background(),
			fctx: func() context.Context {
				ctx := context.Background()
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				return cctx
			}(),
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := &FetchWorker{
				ctx:        tc.fctx,
				readyCh:    tc.readyCh,
				readyErrCh: tc.readyErrCh,
			}
			err := f.WaitForReady(tc.ctx)
			if tc.wantErr != (err != nil) {
				t.Errorf("wantErr %v, but got err: %v", tc.wantErr, err)
			}
		})
	}
}
