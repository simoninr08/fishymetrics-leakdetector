package leak_detector

import (
	"encoding/json"
	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/exporter"
	"github.com/comcast/fishymetrics/pool"
)

type LeakResponse struct {
	Id            string `json:"Id"`
	DetectorState string `json:"DetectorState"`
	Name          string `json:"Name"`
	Status        struct {
		Health string `json:"Health"`
	} `json:"Status"`
}

type LeakPlugin struct {
	DeviceMetrics *map[string]*exporter.Metrics
}

func (l *LeakPlugin) Handler(body []byte) error {
	var data LeakResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return err
	}

	// 1.0 = OK, 0.0 = Alerte
	val := 0.0
	if data.DetectorState == "OK" && data.Status.Health == "OK" {
		val = 1.0
	}

	// Utilisation du groupe "leakMetrics" déclaré dans metrics.go
	m := (*l.DeviceMetrics)["leakMetrics"]
	(*m)["leak_detector_status"].WithLabelValues(data.Name, data.Id).Set(val)

	return nil
}

func (l *LeakPlugin) Apply(e *exporter.Exporter) error {
	l.DeviceMetrics = e.DeviceMetrics
	handlers := []common.Handler{l.Handler}

	endpoints := []string{
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_0_ColdPlate",
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_0_Manifold",
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_1_ColdPlate",
		"/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors/Chassis_0_LeakDetector_1_Manifold",
	}

	for _, url := range endpoints {
		e.GetPool().AddTask(pool.NewTask(e.Fetch(url), handlers))
	}
	return nil
}
