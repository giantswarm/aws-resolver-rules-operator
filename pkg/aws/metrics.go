package aws

import (
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	promprom "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	metricProvider *metric.MeterProvider
)

func init() {
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
	}
	metricProvider = metric.NewMeterProvider(metric.WithReader(exporter))

	// v1 stuff below
	metrics.Registry.MustRegister(awsRequestCount)
	metrics.Registry.MustRegister(awsRequestDurationSeconds)
}

const (
	metricAWSSubsystem       = "aws"
	metricRequestCountKey    = "api_requests_total"
	metricRequestDurationKey = "api_request_duration_seconds"
	metricServiceLabel       = "service"
	metricRegionLabel        = "region"
	metricOperationLabel     = "operation"
	metricControllerLabel    = "controller"
	metricStatusCodeLabel    = "status_code"
	metricErrorCodeLabel     = "error_code"
)

var (
	awsRequestCount = promprom.NewCounterVec(promprom.CounterOpts{
		Subsystem: metricAWSSubsystem,
		Name:      metricRequestCountKey,
		Help:      "Total number of AWS requests",
	}, []string{metricControllerLabel, metricServiceLabel, metricRegionLabel, metricOperationLabel, metricStatusCodeLabel, metricErrorCodeLabel})
	awsRequestDurationSeconds = promprom.NewHistogramVec(promprom.HistogramOpts{
		Subsystem: metricAWSSubsystem,
		Name:      metricRequestDurationKey,
		Help:      "Latency of HTTP requests to AWS",
	}, []string{metricControllerLabel, metricServiceLabel, metricRegionLabel, metricOperationLabel})
)

func captureRequestMetrics(controller string) func(r *request.Request) {
	return func(r *request.Request) {
		duration := time.Since(r.AttemptTime)
		operation := r.Operation.Name
		region := aws.StringValue(r.Config.Region)
		service := endpointToService(r.ClientInfo.Endpoint)
		statusCode := "0"
		errorCode := ""
		if r.HTTPResponse != nil {
			statusCode = strconv.Itoa(r.HTTPResponse.StatusCode)
		}
		if r.Error != nil {
			var ok bool
			if errorCode, ok = toErrorCode(r.Error); !ok {
				errorCode = "internal"
			}
		}
		awsRequestCount.WithLabelValues(controller, service, region, operation, statusCode, errorCode).Inc()
		awsRequestDurationSeconds.WithLabelValues(controller, service, region, operation).Observe(duration.Seconds())
	}
}

func endpointToService(endpoint string) string {
	endpointURL, err := url.Parse(endpoint)
	// If possible extract the service name, else return entire endpoint address
	if err == nil {
		host := endpointURL.Host
		components := strings.Split(host, ".")
		if len(components) > 0 {
			return components[0]
		}
	}
	return endpoint
}

// toErrorCode returns the AWS error code as a string.
// The error code is a short phrase depicting the classification of the error.
func toErrorCode(err error) (string, bool) {
	if awsError, ok := err.(awserr.Error); ok {
		return awsError.Code(), true
	}
	return "", false
}
