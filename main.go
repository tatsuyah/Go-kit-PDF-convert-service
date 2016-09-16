package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/jung-kurt/gofpdf"
	"golang.org/x/net/context"

	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	httptransport "github.com/go-kit/kit/transport/http"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

type ConvertService interface {
	Pdf(string) (string, error)
}

type convertService struct{}

func (convertService) Pdf(s string) (string, error) {
	if s == "" {
		return "", ErrEmpty
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, s)
	pdf.Cell(40, 20, s)
	err := pdf.OutputFileAndClose("example.pdf")
	return "Done", err
}

var ErrEmpty = errors.New("Empty string")

type pdfRequest struct {
	S string `json:"s"`
}

type pdfResponse struct {
	V   string `json:"v"`
	Err string `json:"err,omitempty"`
}

func makePdfEndpoint(svc ConvertService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(pdfRequest)
		v, err := svc.Pdf(req.S)
		if err != nil {
			return pdfResponse{v, err.Error()}, nil
		}
		return pdfResponse{v, ""}, nil
	}
}

func main() {
	ctx := context.Background()
	logger := log.NewLogfmtLogger(os.Stderr)

	fieldKeys := []string{"method", "error"}
	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)
	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)
	countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "count_result",
		Help:      "The result of each count method.",
	}, []string{})

	var svc ConvertService
	svc = convertService{}
	svc = loggingMiddleware{logger, svc}
	svc = instrumentingMiddleware{requestCount, requestLatency, countResult, svc}

	pdfHandler := httptransport.NewServer(
		ctx,
		makePdfEndpoint(svc),
		decodePdfRequest,
		encodeResponse,
	)

	http.Handle("/pdf", pdfHandler)
	logger.Log("msg", "HTTP", "addr", ":8080")
	logger.Log("err", http.ListenAndServe(":8080", nil))
}

func decodePdfRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request pdfRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}
