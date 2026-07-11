package arrowipc

import (
	"context"
	"testing"
)

func FuzzDecodeChartIPC(f *testing.F) {
	f.Add([]byte("not arrow"), ChartSchemaVersion, uint8(ChartContinuous))
	f.Add([]byte{}, 999, uint8(ChartCandles))
	f.Fuzz(func(t *testing.T, data []byte, version int, kind uint8) {
		_, _ = DecodeChart(context.Background(), data, version, ChartKind(kind))
	})
}
