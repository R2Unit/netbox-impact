package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type ImpactType string

// Weight points voor het impact_type
const (
	PlannedWork    ImpactType = "planned-work"
	FiberWorks     ImpactType = "fiber-works"
	ElectricalWork ImpactType = "electrical-work"
	IncidentWork   ImpactType = "incident-work"
)

var ImpactTypeWeights = map[ImpactType]float64{
	PlannedWork:    1.0,
	FiberWorks:     1.5,
	ElectricalWork: 2.0,
	IncidentWork:   10.0,
}

type ImpactRequest struct {
	DeviceIDs    []int      `json:"device_ids"`
	CircuitIDs   []int      `json:"circuit_ids"`
	InterfaceIDs []int      `json:"interface_ids"`
	ImpactType   ImpactType `json:"impact_type"`
}

type Node struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Device struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Circuit struct {
	ID           int    `json:"id"`
	CID          string `json:"cid"`
	TerminationA Node   `json:"termination_a"`
	TerminationB Node   `json:"termination_b"`
}

type Interface struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Device string `json:"device"`
}

type NetboxClient struct {
	APIUrl string
	Token  string
	Client *http.Client
}

func NewNetboxClient(apiUrl, token string) *NetboxClient {
	return &NetboxClient{
		APIUrl: apiUrl,
		Token:  token,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *NetboxClient) fetch(endpoint string, v interface{}) error {
	url := c.APIUrl + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch %s: status %d", endpoint, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *NetboxClient) FetchDevices() ([]Device, error) {
	var result struct {
		Results []Device `json:"results"`
	}
	err := c.fetch("/api/dcim/devices/", &result)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (c *NetboxClient) FetchCircuits() ([]Circuit, error) {
	var result struct {
		Results []Circuit `json:"results"`
	}
	err := c.fetch("/api/circuits/circuits/", &result)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (c *NetboxClient) FetchInterfaces() ([]Interface, error) {
	var result struct {
		Results []Interface `json:"results"`
	}
	err := c.fetch("/api/dcim/interfaces/", &result)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (c *NetboxClient) FetchCircuitByID(id int) (*Circuit, error) {
	endpoint := fmt.Sprintf("/api/circuits/circuits/%d/", id)
	var circuit Circuit
	err := c.fetch(endpoint, &circuit)
	if err != nil {
		return nil, err
	}
	return &circuit, nil
}

func redundancyFactorCircuit(c Circuit) float64 {
	if c.TerminationA.ID == c.TerminationB.ID {
		return 0.8
	}
	return 1.0
}

type DeviceImpact struct {
	Count           int     `json:"count"`
	WeightPerDevice float64 `json:"weight_per_device"`
	Impact          float64 `json:"impact"`
}

type CircuitImpactDetail struct {
	ID               int     `json:"id"`
	CID              string  `json:"cid"`
	RedundancyFactor float64 `json:"redundancy_factor"`
	Weight           float64 `json:"weight"`
	Impact           float64 `json:"impact"`
}

type CircuitImpact struct {
	Items       []CircuitImpactDetail `json:"items"`
	TotalImpact float64               `json:"total_impact"`
}

type InterfaceImpact struct {
	Count              int     `json:"count"`
	WeightPerInterface float64 `json:"weight_per_interface"`
	Impact             float64 `json:"impact"`
}

type ImpactBreakdown struct {
	Devices         DeviceImpact    `json:"devices"`
	ImplicitDevices DeviceImpact    `json:"implicit_devices"`
	Circuits        CircuitImpact   `json:"circuits"`
	Interfaces      InterfaceImpact `json:"interfaces"`
}

type ImpactResult struct {
	TotalImpact                 float64         `json:"total_impact"`
	TotalImpactBeforeMultiplier float64         `json:"total_impact_before_multiplier"`
	Multiplier                  float64         `json:"multiplier"`
	Breakdown                   ImpactBreakdown `json:"breakdown"`
}

func CalculateImpactDetailed(req ImpactRequest, client *NetboxClient) (ImpactResult, error) {
	deviceWeight := 5.0
	circuitWeight := 3.0
	interfaceWeight := 1.0

	deviceCount := len(req.DeviceIDs)
	deviceImpact := float64(deviceCount) * deviceWeight

	interfaceCount := len(req.InterfaceIDs)
	interfaceImpact := float64(interfaceCount) * interfaceWeight

	var circuitDetails []CircuitImpactDetail
	totalCircuitImpact := 0.0
	implicitDeviceIDs := make(map[int]bool)

	for _, cid := range req.CircuitIDs {
		circuit, err := client.FetchCircuitByID(cid)
		if err != nil {
			return ImpactResult{}, fmt.Errorf("failed to fetch circuit %d: %v", cid, err)
		}
		rf := redundancyFactorCircuit(*circuit)
		impact := circuitWeight * rf
		detail := CircuitImpactDetail{
			ID:               circuit.ID,
			CID:              circuit.CID,
			RedundancyFactor: rf,
			Weight:           circuitWeight,
			Impact:           impact,
		}
		circuitDetails = append(circuitDetails, detail)
		totalCircuitImpact += impact

		if rf < 1.0 {
			implicitDeviceIDs[circuit.TerminationA.ID] = true
		}
	}

	implicitDeviceCount := len(implicitDeviceIDs)
	implicitDeviceImpact := float64(implicitDeviceCount) * deviceWeight

	totalBeforeMultiplier := deviceImpact + implicitDeviceImpact + totalCircuitImpact + interfaceImpact

	multiplier, ok := ImpactTypeWeights[req.ImpactType]
	if !ok {
		multiplier = 1.0
	}
	totalImpact := multiplier * totalBeforeMultiplier

	result := ImpactResult{
		TotalImpact:                 totalImpact,
		TotalImpactBeforeMultiplier: totalBeforeMultiplier,
		Multiplier:                  multiplier,
		Breakdown: ImpactBreakdown{
			Devices: DeviceImpact{
				Count:           deviceCount,
				WeightPerDevice: deviceWeight,
				Impact:          deviceImpact,
			},
			ImplicitDevices: DeviceImpact{
				Count:           implicitDeviceCount,
				WeightPerDevice: deviceWeight,
				Impact:          implicitDeviceImpact,
			},
			Circuits: CircuitImpact{
				Items:       circuitDetails,
				TotalImpact: totalCircuitImpact,
			},
			Interfaces: InterfaceImpact{
				Count:              interfaceCount,
				WeightPerInterface: interfaceWeight,
				Impact:             interfaceImpact,
			},
		},
	}
	return result, nil
}

func ImpactMiddleware(client *NetboxClient, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/calculateImpact" && r.Method == http.MethodPost {
			var req ImpactRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request payload", http.StatusBadRequest)
				return
			}
			result, err := CalculateImpactDetailed(req, client)
			if err != nil {
				http.Error(w, "Error calculating impact: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func runCLI(client *NetboxClient) {
	reader := bufio.NewReader(os.Stdin)

	devices, err := client.FetchDevices()
	if err != nil {
		log.Fatalf("Error fetching devices: %v", err)
	}
	fmt.Println("Available Devices:")
	for _, d := range devices {
		fmt.Printf("ID: %d, Name: %s\n", d.ID, d.Name)
	}
	fmt.Print("Enter device IDs (comma-separated): ")
	deviceInput, _ := reader.ReadString('\n')
	deviceIDs := parseIDs(deviceInput)

	circuits, err := client.FetchCircuits()
	if err != nil {
		log.Fatalf("Error fetching circuits: %v", err)
	}
	fmt.Println("\nAvailable Circuits:")
	for _, c := range circuits {
		fmt.Printf("ID: %d, CID: %s, TerminationA: %s, TerminationB: %s\n",
			c.ID, c.CID, c.TerminationA.Name, c.TerminationB.Name)
	}
	fmt.Print("Enter circuit IDs (comma-separated): ")
	circuitInput, _ := reader.ReadString('\n')
	circuitIDs := parseIDs(circuitInput)

	interfaces, err := client.FetchInterfaces()
	if err != nil {
		log.Fatalf("Error fetching interfaces: %v", err)
	}
	fmt.Println("\nAvailable Interfaces:")
	for _, i := range interfaces {
		fmt.Printf("ID: %d, Name: %s, Device: %s\n", i.ID, i.Name, i.Device)
	}
	fmt.Print("Enter interface IDs (comma-separated): ")
	interfaceInput, _ := reader.ReadString('\n')
	interfaceIDs := parseIDs(interfaceInput)

	fmt.Print("\nEnter impact type (planned-work, fiber-works, electrical-work, incident-work): ")
	impactTypeInput, _ := reader.ReadString('\n')
	impactTypeInput = strings.TrimSpace(impactTypeInput)
	var impactType ImpactType = ImpactType(impactTypeInput)

	req := ImpactRequest{
		DeviceIDs:    deviceIDs,
		CircuitIDs:   circuitIDs,
		InterfaceIDs: interfaceIDs,
		ImpactType:   impactType,
	}
	result, err := CalculateImpactDetailed(req, client)
	if err != nil {
		log.Fatalf("Error calculating impact: %v", err)
	}
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("\nDetailed Impact Result:\n%s\n", string(resultJSON))
}

func parseIDs(input string) []int {
	var ids []int
	parts := strings.Split(strings.TrimSpace(input), ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if id, err := strconv.Atoi(part); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func main() {
	mode := flag.String("mode", "server", "Mode to run: server or cli")
	netboxURL := flag.String("netbox-url", "http://localhost:8000", "NetBox API URL")
	netboxToken := flag.String("netbox-token", "YOUR_NETBOX_TOKEN", "NetBox API token")
	flag.Parse()

	client := NewNetboxClient(*netboxURL, *netboxToken)

	if *mode == "cli" {
		runCLI(client)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Netbox Impact API"))
	})

	handler := ImpactMiddleware(client, mux)
	log.Println("Server running on HTTP port (80)")
	if err := http.ListenAndServe(":80", handler); err != nil {
		log.Fatal(err)
	}
}
