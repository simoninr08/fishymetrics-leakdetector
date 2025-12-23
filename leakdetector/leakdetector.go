package leakdetector

import (
	"encoding/json"
	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/exporter" // Vérifie que ce chemin est correct
	"github.com/comcast/fishymetrics/pool"
)

type LeakResponse struct {
	Id            string `json:"Id"`
	DetectorState string `json:"DetectorState"`
	Name          string `json:"Name"`
	Status        struct { Health string `json:"Health"` } `json:"Status"`
}

type LeakPlugin struct {
	// Utilise le type générique si exporter.Metrics pose problème
	DeviceMetrics *map[string]*exporter.Metrics 
}

func (l *LeakPlugin) Handler(body []byte) error {
	var data LeakResponse
	if err := json.Unmarshal(body, &data); err != nil { return err }

	val := 0.0
	if data.DetectorState == "OK" && data.Status.Health == "OK" { val = 1.0 }

	m := (*l.DeviceMetrics)["leakMetrics"]
	// On cast en prometheus.GaugeVec si nécessaire, ou on utilise l'interface existante
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

	for _, uri := range endpoints {
		// CORRECTION NewTask : il faut 3 arguments
		// 1. La fonction à exécuter : e.Get(uri) ou e.Fetch(uri)
		// 2. Un nom pour la tâche (uri)
		// 3. Les handlers
		task := pool.NewTask(func() ([]byte, error) {
			return e.Get(uri) // Vérifie si la méthode dans exporter est Get ou Fetch
		}, uri, handlers)
		
		e.GetPool().AddTask(task)
	}
	return nil
}
