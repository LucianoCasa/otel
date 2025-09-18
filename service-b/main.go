package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"service-b/tracing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/joho/godotenv"
)

type CepService struct {
	BaseURL    string
	HTTPClient *http.Client
}

type WeatherService struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

var (
	cepBaseURL     = "https://viacep.com.br/ws"
	weatherBaseURL = "http://api.weatherapi.com/v1"
	cepRegex       = regexp.MustCompile(`^\d{8}$`)

	ErrMsgCepNotFound = "can not find zipcode"
	ErrMsgInvalidCep  = "invalid zipcode"
	ErrMsgCepAPI      = "cep status not ok"
	ErrMsgWeatherAPI  = "weather status not ok"

	ErrCepNotFound = errors.New(ErrMsgCepNotFound)
	ErrCepAPI      = errors.New(ErrMsgCepAPI)
	ErrWeatherAPI  = errors.New(ErrMsgWeatherAPI)
	ErrWeather     = errors.New("weather error")

	ServiceBListening = "Service B listening on "
)

func main() {
	_ = godotenv.Load()
	weatherAPIKey := os.Getenv("WEATHERAPI_KEY")
	port := os.Getenv("SERVICE_B_PORT")
	if port == "" {
		port = "8081"
	}

	shutdown := tracing.InitTracer("service-b")
	defer shutdown()

	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	cepSvc := &CepService{BaseURL: cepBaseURL, HTTPClient: httpClient}
	weatherSvc := &WeatherService{BaseURL: weatherBaseURL, APIKey: weatherAPIKey, HTTPClient: httpClient}

	mux := http.NewServeMux()
	mux.Handle("/weather", otelhttp.NewHandler(weatherHandler(cepSvc, weatherSvc), "GET /weather"))

	log.Printf("%s :%s", ServiceBListening, port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func (c *CepService) Lookup(ctx context.Context, cep string) (string, error) {
	tr := otel.Tracer("cep")
	ctx, span := tr.Start(ctx, "viaCEP.lookup")
	defer span.End()
	span.SetAttributes(attribute.String("cep", cep))

	type cepResp struct {
		Localidade string `json:"localidade"`
		Erro       bool   `json:"erro"`
	}
	u := fmt.Sprintf("%s/%s/json/", c.BaseURL, cep)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		return "", ErrCepAPI
	}
	var data cepResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.Erro || data.Localidade == "" {
		return "", ErrCepNotFound
	}
	span.SetAttributes(attribute.String("city", data.Localidade))
	return data.Localidade, nil
}

func (wSvc *WeatherService) GetTempC(ctx context.Context, city string) (float64, error) {
	tr := otel.Tracer("weather")
	ctx, span := tr.Start(ctx, "weatherAPI.current")
	defer span.End()
	span.SetAttributes(attribute.String("city", city))

	u := fmt.Sprintf("%s/current.json?key=%s&q=%s",
		wSvc.BaseURL, wSvc.APIKey, url.QueryEscape(city))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := wSvc.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		return 0, ErrWeatherAPI
	}
	var wData struct {
		Current struct {
			TempC float64 `json:"temp_c"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wData); err != nil {
		return 0, err
	}
	return wData.Current.TempC, nil
}

func weatherHandler(cepSvc *CepService, weatherSvc *WeatherService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cep := r.URL.Query().Get("cep")
		if !cepRegex.MatchString(cep) {
			http.Error(w, ErrMsgInvalidCep, http.StatusUnprocessableEntity)
			return
		}
		city, err := cepSvc.Lookup(r.Context(), cep)
		if err != nil {
			http.Error(w, ErrMsgCepNotFound, http.StatusNotFound)
			return
		}
		tempC, err := weatherSvc.GetTempC(r.Context(), city)
		if err != nil {
			http.Error(w, ErrWeather.Error(), http.StatusInternalServerError)
			return
		}
		resp := map[string]any{
			"city":   city,
			"temp_C": tempC,
			"temp_F": tempC*1.8 + 32,
			"temp_K": tempC + 273,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
