package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"otel/pkg/tracing"
	"regexp"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"encoding/json"
)

var cepRegex = regexp.MustCompile(`^\d{8}$`)

type cepReq struct {
	CEP string `json:"cep"`
}

var (
	ErrMethodNotAllowed    = "method not allowed"
	ErrInvalidBody         = "invalid body"
	ErrMsgInvalidCep       = "invalid cep"
	ErrServiceAUnreachable = "service-a unreachable"

	ServiceAListening = "Service A listening on "
)

func main() {
	port := os.Getenv("SERVICE_A_PORT")
	if port == "" {
		port = "8080"
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}

	bURL := os.Getenv("SERVICE_B_URL")
	if bURL == "" {
		bURL = "http://service-b:8081/weather"
	}

	shutdown := tracing.InitTracer("service-a", endpoint)
	defer shutdown()

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, ErrMethodNotAllowed, http.StatusMethodNotAllowed)
			return
		}
		var in cepReq
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, ErrInvalidBody, http.StatusBadRequest)
			return
		}
		if !cepRegex.MatchString(in.CEP) {
			http.Error(w, ErrMsgInvalidCep, http.StatusUnprocessableEntity)
			return
		}

		req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, bURL+"?cep="+in.CEP, nil)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, ErrServiceAUnreachable, http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}

	mux := http.NewServeMux()
	mux.Handle("/weather", otelhttp.NewHandler(http.HandlerFunc(handler), "POST /weather"))

	log.Printf("%s :%s", ServiceAListening, port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
