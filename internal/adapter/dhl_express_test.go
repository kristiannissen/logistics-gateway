// Package adapter provides interfaces and implementations for carrier integrations.
// This file is located at /internal/adapter/dhl_express_test.go.
package adapter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newDHLExpressTestServer spins up an httptest.Server shaped like the MyDHL
// API and returns a DHLExpressAdapter pointed at it, along with the captured
// request path, method, and JSON body of the last request handled.
func newDHLExpressTestServer(t *testing.T, statusCode int, body string) (*DHLExpressAdapter, **http.Request, *map[string]any) {
	t.Helper()

	var captured map[string]any
	var lastReq *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if len(raw) > 0 {
			require.NoError(t, json.Unmarshal(raw, &captured))
		}
		lastReq = r
		w.WriteHeader(statusCode)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)

	adapter := NewDHLExpressAdapter("test-user", "test-pass", "123456789", "P", "R", zap.NewNop())
	adapter.BaseURL = srv.URL
	adapter.HTTPClient = srv.Client()

	return adapter, &lastReq, &captured
}

// =========================================================================
// UpdateShipment — add-piece (APIdocs/dhl_express.md, PATCH .../add-piece)
// =========================================================================

func TestDHLExpressAdapter_UpdateShipment_AddPiece(t *testing.T) {
	t.Parallel()

	t.Run("wires add-piece request and succeeds on 200", func(t *testing.T) {
		t.Parallel()
		adapter, lastReq, captured := newDHLExpressTestServer(t, http.StatusOK, "")

		resp, err := adapter.UpdateShipment(t.Context(), UpdateRequest{
			TrackingNumber: "9356579890",
			AddPiece: &AddPieceRequest{
				Weight:      2.5,
				Dimensions:  Dimensions{Length: 30, Width: 20, Height: 15},
				Reference:   "colli-2",
				Description: "Extra piece",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "9356579890", resp.TrackingNumber)
		assert.Equal(t, "updated", resp.Status)
		assert.Equal(t, []string{"addPiece"}, resp.UpdatedFields)

		require.NotNil(t, *lastReq)
		assert.Equal(t, http.MethodPatch, (*lastReq).Method)
		assert.Contains(t, (*lastReq).URL.Path, "/shipments/9356579890/add-piece")

		payload := *captured
		content, ok := payload["content"].(map[string]any)
		require.True(t, ok)
		packages, ok := content["packages"].([]any)
		require.True(t, ok)
		require.Len(t, packages, 1)
		pkg := packages[0].(map[string]any)
		assert.InEpsilon(t, 2.5, pkg["weight"], 0.0001)
		assert.Equal(t, "Extra piece", pkg["description"])
	})

	t.Run("uses caller-supplied originalPlannedShippingDate", func(t *testing.T) {
		t.Parallel()
		adapter, _, captured := newDHLExpressTestServer(t, http.StatusOK, "")

		_, err := adapter.UpdateShipment(t.Context(), UpdateRequest{
			TrackingNumber: "9356579890",
			AddPiece: &AddPieceRequest{
				Weight:                      1.0,
				OriginalPlannedShippingDate: "2020-04-20",
			},
		})
		require.NoError(t, err)

		payload := *captured
		assert.Equal(t, "2020-04-20", payload["originalPlannedShippingDate"])
	})

	t.Run("rejects when AddPiece is not set", func(t *testing.T) {
		t.Parallel()
		adapter, _, _ := newDHLExpressTestServer(t, http.StatusOK, "")

		_, err := adapter.UpdateShipment(t.Context(), UpdateRequest{
			TrackingNumber: "9356579890",
			ReceiverPhone:  "+4587654321",
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNotSupported)
	})

	t.Run("rejects zero weight", func(t *testing.T) {
		t.Parallel()
		adapter, _, _ := newDHLExpressTestServer(t, http.StatusOK, "")

		_, err := adapter.UpdateShipment(t.Context(), UpdateRequest{
			TrackingNumber: "9356579890",
			AddPiece:       &AddPieceRequest{Weight: 0},
		})
		require.Error(t, err)
	})

	t.Run("rejects missing tracking number", func(t *testing.T) {
		t.Parallel()
		adapter, _, _ := newDHLExpressTestServer(t, http.StatusOK, "")

		_, err := adapter.UpdateShipment(t.Context(), UpdateRequest{
			AddPiece: &AddPieceRequest{Weight: 1.0},
		})
		require.Error(t, err)
	})

	t.Run("propagates carrier error on non-2xx", func(t *testing.T) {
		t.Parallel()
		adapter, _, _ := newDHLExpressTestServer(t, http.StatusNotFound,
			`{"detail":"7163: Operation not possible since shipment information not found"}`)

		_, err := adapter.UpdateShipment(t.Context(), UpdateRequest{
			TrackingNumber: "9356579890",
			AddPiece:       &AddPieceRequest{Weight: 1.0},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "add-piece failed")
	})
}

// =========================================================================
// Mock adapter — UpdateShipment
// =========================================================================

func TestMockDHLExpressAdapter_UpdateShipment(t *testing.T) {
	t.Parallel()

	t.Run("accepts AddPiece", func(t *testing.T) {
		t.Parallel()
		resp, err := (&MockDHLExpressAdapter{}).UpdateShipment(t.Context(), UpdateRequest{
			TrackingNumber: "1234567890",
			AddPiece:       &AddPieceRequest{Weight: 1.5},
		})
		require.NoError(t, err)
		assert.Equal(t, "updated", resp.Status)
		assert.Equal(t, []string{"addPiece"}, resp.UpdatedFields)
	})

	t.Run("rejects everything else", func(t *testing.T) {
		t.Parallel()
		_, err := (&MockDHLExpressAdapter{}).UpdateShipment(t.Context(), UpdateRequest{
			TrackingNumber: "1234567890",
			ReceiverEmail:  "new@example.com",
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNotSupported)
	})
}
