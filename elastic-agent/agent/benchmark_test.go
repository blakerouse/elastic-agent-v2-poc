package agent

import (
	"context"
	"github.com/elastic/elastic-agent/pipeline"
	"testing"

	"github.com/rs/zerolog"
)

func BenchmarkInside(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	b.ReportAllocs()
	mgr, err := NewInsideManager()
	if err != nil {
		b.Fatal(err)
	}

	_, err = mgr.Run(context.Background())
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkDirectEncoded(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	b.ReportAllocs()
	mgr, err := NewDirectManager()
	if err != nil {
		b.Fatal(err)
	}

	_, err = mgr.Run(context.Background(), pipeline.EncodeSend{})
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkDirect(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	b.ReportAllocs()
	mgr, err := NewDirectManager()
	if err != nil {
		b.Fatal(err)
	}

	_, err = mgr.Run(context.Background(), pipeline.DirectSend{})
	if err != nil {
		b.Fatal(err)
	}
}
