package statuschecks

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"
	"time"

	"github.com/facebookgo/httpcontrol"
)

const (
	Healthy  = "healthy"
	Degraded = "degraded"
	Down     = "down"
)

// StatusRep represents the status of a given system and its Subsystems. Note
// that Version & Subsystems may be nil.
type StatusRep struct {
	ServiceStatus
	Version    string     `json:"version,omitempty"`
	Subsystems Subsystems `json:"subsystems"`
}

type ServiceStatus struct {
	Status string `json:"status"`
}

type Subsystems struct {
	Critical    map[string]StatusRep `json:"critical"`
	Noncritical map[string]StatusRep `json:"noncritical"`
}

var client *http.Client

func init() {
	baseTransport := httpcontrol.Transport{
		RequestTimeout: 2 * time.Second,
		DialTimeout:    2 * time.Second,
		DialKeepAlive:  1 * time.Minute,
		MaxTries:       3,
	}

	client = &http.Client{
		Transport: &baseTransport,
	}
}

func (s ServiceStatus) IsHealthy() bool {
	return s.Status == Healthy
}

func (s ServiceStatus) IsDegraded() bool {
	return s.Status == Degraded
}

func (s ServiceStatus) IsDown() bool {
	return s.Status == Down
}

func (s Subsystems) Status() ServiceStatus {
	status := HealthyService()
	for _, service := range s.Critical {
		if service.IsDown() {
			status = DownService()
			break
		} else if service.IsDegraded() {
			status = DegradedService()
		}
	}

	return status
}

func HealthyService() ServiceStatus {
	return ServiceStatus{
		Status: Healthy,
	}
}
func DownService() ServiceStatus {
	return ServiceStatus{
		Status: Down,
	}
}
func DegradedService() ServiceStatus {
	return ServiceStatus{
		Status: Degraded,
	}
}

func DownServiceRep() StatusRep {
	return StatusRep{
		ServiceStatus: DownService(),
	}
}
func DegradedServiceRep() StatusRep {
	return StatusRep{
		ServiceStatus: DegradedService(),
	}
}

func CheckStatus(resource string) StatusRep {
	req, reqErr := http.NewRequest("GET", resource, nil)

	if reqErr != nil {
		return DownServiceRep()
	}
	req.Header.Set("User-Agent", "LD/HealthCheck")
	req.Header.Set("X-LD-Private", "allowed")

	res, resErr := client.Do(req)

	if resErr != nil {
		return DownServiceRep()
	} else {
		defer res.Body.Close()
	}
	if res.StatusCode >= 400 {
		return DegradedServiceRep()
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return DownServiceRep()
	}
	var status StatusRep
	jsonErr := json.Unmarshal(body, &status)
	if jsonErr != nil {
		return DownServiceRep()
	}
	return status
}

func CheckGoroutines(threshold int) ServiceStatus {
	if runtime.NumGoroutine() > threshold {
		return DegradedService()
	} else {
		return HealthyService()
	}
}
