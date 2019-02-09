package inventory

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/opentracing/opentracing-go"
	otrext "github.com/opentracing/opentracing-go/ext"

	. "wavefront.com/polyglot/inventory/internal"
)

type InventoryService struct {
	HostURL string
	Router  *chi.Mux
	tracer  opentracing.Tracer
}

func NewServer() Server {
	r := chi.NewRouter()
	server := &InventoryService{GlobalConfig.InventoryHost, r, opentracing.GlobalTracer()}
	r.Route("/inventory", func(r chi.Router) {
		r.Get("/available/{itemId}", server.available)
		r.Post("/checkout/{orderId}", server.checkout)
	})
	return server
}

func (s *InventoryService) Start() error {
	log.Printf("Inventory service listening on: %s", s.HostURL)
	return http.ListenAndServe(s.HostURL, s.Router)
}

func (s *InventoryService) available(w http.ResponseWriter, r *http.Request) {
	span := NewServerSpan(r, "available")
	defer span.Finish()

	RandSimDelay()

	if RAND.Float32() < GlobalConfig.SimFailDelivery1 {
		otrext.Error.Set(span, true)
		WriteError(w, "Failed to check availability", http.StatusServiceUnavailable)
		return
	}

	exists := true
	if RAND.Float32() < GlobalConfig.SimFailDelivery2 {
		exists = false
	}

	if !exists {
		otrext.Error.Set(span, true)
		WriteError(w, "Item does not exist", http.StatusNotFound)
		return
	}
	w.Write([]byte{byte(http.StatusOK)})
}

func (s *InventoryService) checkout(w http.ResponseWriter, r *http.Request) {
	span := s.tracer.StartSpan("checkout")
	defer span.Finish()

	RandSimDelay()

	if RAND.Float32() < GlobalConfig.SimFailCheckout {
		otrext.Error.Set(span, true)
		WriteError(w, "checkout failure", http.StatusServiceUnavailable)
		return
	}

	resp, err := callWarehouse(span.Context())
	if err != nil {
		otrext.Error.Set(span, true)
		WriteError(w, err.Error(), http.StatusPreconditionFailed)
		return
	}
	defer resp.Body.Close()

	RandSimDelay()

	if resp.StatusCode == http.StatusOK {
		io.Copy(w, resp.Body)
	} else {
		otrext.Error.Set(span, true)
		WriteError(w, fmt.Sprintf("failed to checkout: %s", resp.Status), resp.StatusCode)
	}
}

func callWarehouse(spanCtx opentracing.SpanContext) (*http.Response, error) {
	getURL := fmt.Sprintf("http://%s/warehouse/%s", GlobalConfig.WarehouseHost, "32jf")
	return GETCall(getURL, nil, spanCtx)
}
