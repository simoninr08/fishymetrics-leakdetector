package leak_detector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/exporter"
	"github.com/comcast/fishymetrics/pool"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/client_golang/prometheus"
)

// LeakResponse represents the JSON structure returned by the Redfish LeakDetector endpoint
type LeakResponse struct {
	Name          string `json:"Name"`
	Id            string `json:"Id"`
	DetectorState string `json:"DetectorState"`
	Status        struct {
		State  string `json:"State"`
		Health string `json:"Health"`
	} `json:"Status"`
}

type LeakPlugin struct {
	DeviceMetrics interface{}
}

// Apply initializes the plugin, extracts credentials, and schedules the Redfish scrape tasks
func (l *LeakPlugin) Apply(e *exporter.Exporter) error {
	l.DeviceMetrics = e.DeviceMetrics

	// Extract credentials directly from command line arguments
	user, pass := l.getAuthFromArgs()

	endpoints := []string{
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_0_ColdPlate",
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_0_Manifold",
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_1_ColdPlate",
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_1_Manifold",
	}

	handlers := []common.Handler{l.Handler}

	for _, uri := range endpoints {
		u := uri
		task := pool.NewTask(func() ([]byte, error) {
			fullUrl := fmt.Sprintf("%s%s", e.GetUrl(), u)
			req, _ := http.NewRequestWithContext(e.GetContext(), "GET", fullUrl, nil)

			// Apply Basic Auth if credentials were found in args
			if user != "" {
				req.SetBasicAuth(user, pass)
			}

			retryReq, _ := retryablehttp.FromRequest(req)
			resp, err := e.GetClient().Do(retryReq)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				return nil, fmt.Errorf("Status %d", resp.StatusCode)
			}
			return io.ReadAll(resp.Body)
		}, u, handlers)
		e.GetPool().AddTask(task)
	}
	return nil
}

// getAuthFromArgs scans os.Args to find --user and --password flags
func (l *LeakPlugin) getAuthFromArgs() (string, string) {
	var user, pass string
	args := os.Args
	for i, arg := range args {
		// Handle --user=admin or -user admin
		if strings.HasPrefix(arg, "--user=") {
			user = strings.Split(arg, "=")[1]
		} else if arg == "--user" || arg == "-user" {
			if i+1 < len(args) {
				user = args[i+1]
			}
		}
		// Handle --password=admin or -password admin
		if strings.HasPrefix(arg, "--password=") {
			pass = strings.Split(arg, "=")[1]
		} else if arg == "--password" || arg == "-password" {
			if i+1 < len(args) {
				pass = args[i+1]
			}
		}
	}
	return user, pass
}

// Handler processes the HTTP response body and updates the Prometheus metrics
func (l *LeakPlugin) Handler(body []byte) error {
	var data LeakResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}

	// Logic: 1.0 if state and health are OK, 0.0 otherwise
	val := 0.0
	if data.DetectorState == "OK" && data.Status.Health == "OK" {
		val = 1.0
	}

	if l.DeviceMetrics == nil {
		return nil
	}

	// Use reflection to access the private 'leakMetrics' map in the exporter
	v := reflect.ValueOf(l.DeviceMetrics)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Map {
		return nil
	}

	groupKey := reflect.ValueOf("leakMetrics")
	groupVal := v.MapIndex(groupKey)
	if !groupVal.IsValid() {
		return nil
	}

	// Handle pointer or interface within the map
	if groupVal.Kind() == reflect.Ptr || groupVal.Kind() == reflect.Interface {
		groupVal = groupVal.Elem()
	}

	if groupVal.Kind() == reflect.Map {
		metricKey := reflect.ValueOf("leak_detector_status")
		gaugeVal := groupVal.MapIndex(metricKey)
		if gaugeVal.IsValid() {
			// Assert that the interface is a *prometheus.GaugeVec
			if gauge, ok := gaugeVal.Interface().(*prometheus.GaugeVec); ok && gauge != nil {
				gauge.WithLabelValues(data.Name, data.Id).Set(val)
			}
		}
	}
	return nil
}

