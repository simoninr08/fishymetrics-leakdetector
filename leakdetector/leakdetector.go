package leakdetector

import (
    "context"
    "encoding/json"

    "github.com/comcast/fishymetrics/exporter"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type LeakDetectorsCollection struct {
    Members []struct {
        OdataId string `json:"@odata.id"`
    } `json:"Members"`
}

type LeakDetector struct {
    Id              string `json:"Id"`
    LeakDetectorType string `json:"LeakDetectorType"`
    Status struct {
        Health string `json:"Health"`
        State  string `json:"State"`
    } `json:"Status"`
}

var (
    leakHealth = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "fishymetrics_leak_detector_health",
        Help: "Redfish leak detector health (0=OK,1=Warn,2=Crit)",
    }, []string{"detector_id", "type"})

    leakState = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "fishymetrics_leak_detector_state",
        Help: "Redfish leak detector enabled state",
    }, []string{"detector_id", "type"})
)

func healthToValue(h string) float64 {
    switch h {
    case "OK": return 0
    case "Warning": return 1
    case "Critical": return 2
    }
    return -1
}

func stateToValue(s string) float64 {
    switch s {
    case "Enabled": return 1
    case "Disabled": return 0
    }
    return -1
}

// InitPlugin called by Fishymetrics
func InitPlugin(ctx context.Context, ex *exporter.Exporter) error {
    ex.Logger.Info("LeakDetector plugin started")

    client := ex.HTTPClient

    collectionURL := "/redfish/v1/Chassis/Chassis_0/ThermalSubsystem/LeakDetection/LeakDetectors"

    resp, err := client.Get(collectionURL)
    if err != nil {
        ex.Logger.Errorf("failed to fetch leak detectors: %v", err)
        return err
    }
    defer resp.Body.Close()

    var coll LeakDetectorsCollection
    json.NewDecoder(resp.Body).Decode(&coll)

    for _, m := range coll.Members {
        r2, err := client.Get(m.OdataId)
        if err != nil {
            ex.Logger.Errorf("failed fetch %s: %v", m.OdataId, err)
            continue
        }
        defer r2.Body.Close()

        var det LeakDetector
        json.NewDecoder(r2.Body).Decode(&det)

        leakHealth.WithLabelValues(det.Id, det.LeakDetectorType).Set(healthToValue(det.Status.Health))
        leakState.WithLabelValues(det.Id, det.LeakDetectorType).Set(stateToValue(det.Status.State))
    }

    return nil
}

