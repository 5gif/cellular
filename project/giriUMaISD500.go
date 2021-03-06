package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/wiless/cellular/pathloss"

	cell "github.com/wiless/cellular"

	"github.com/wiless/cellular/antenna"
	"github.com/wiless/cellular/deployment"
	"github.com/wiless/vlib"
)

var matlab *vlib.Matlab
var defaultAAS antenna.SettingAAS
var templateAAS map[int]*antenna.SettingAAS

var singlecell deployment.DropSystem
var secangles = vlib.VectorF{0.0, 120.0, -120.0}
var nSectors = 1
var CellRadius = 250.0
var nUEPerCell = 550
var nCells = 19
var CarriersGHz = vlib.VectorF{0.4, 1.8}

func init() {

	defaultAAS.SetDefault()
	defaultAAS.N = 1
	defaultAAS.FreqHz = CarriersGHz[0]
	defaultAAS.BeamTilt = 0
	defaultAAS.DisableBeamTit = false
	defaultAAS.VTiltAngle = 30
	defaultAAS.ESpacingVFactor = .5
	defaultAAS.HTiltAngle = 0
	defaultAAS.MfileName = "output.m"
	defaultAAS.Omni = true
	defaultAAS.GainDb = 10
	defaultAAS.HoldOn = false
	defaultAAS.AASArrayType = antenna.LinearPhaseArray
	defaultAAS.CurveWidthInDegree = 30.0
	defaultAAS.CurveRadius = 1.00

	vlib.SaveStructure(defaultAAS, "defaultAAS.json", true)
}

func main() {
	matlab = vlib.NewMatlab("deployment")
	matlab.Silent = true
	matlab.Json = true

	seedvalue := time.Now().Unix()
	/// comment the below line to have different seed everytime
	seedvalue = 0
	rand.Seed(seedvalue)

	var hatamodel pathloss.OkumuraHata
	// var fishmodel richard.FishModel

	DeployLayer1(&singlecell)

	// singlecell.SetAllNodeProperty("BS", "AntennaType", 0)
	// singlecell.SetAllNodeProperty("UE", "AntennaType", 1)       /// Set All Pico to use antenna Type 1
	singlecell.SetAllNodeProperty("BS", "FreqGHz", CarriersGHz) /// Set All Pico to use antenna Type 0
	singlecell.SetAllNodeProperty("UE", "FreqGHz", CarriersGHz) /// Set All Pico to use antenna Type 0

	rxids := singlecell.GetNodeIDs("UE")
	RxMetrics400 := make(map[int]cell.LinkMetric)

	/// System 1 @ 400MHz

	wsystem := cell.NewWSystem()
	wsystem.BandwidthMHz = 10
	wsystem.FrequencyGHz = 1.8

	singlecell.SetAllNodeProperty("BS", "TxPowerDBm", 44.0)
	for _, rxid := range rxids {

		metric := wsystem.EvaluteLinkMetric(&singlecell, &hatamodel, rxid, myfunc)
		RxMetrics400[rxid] = metric

	}
	vlib.SaveStructure(RxMetrics400, "metric400MHz.json", true)

	/// System 2 @ 1800MHz
	RxMetrics1800 := make(map[int]cell.LinkMetric)
	wsystem.BandwidthMHz = 10
	wsystem.FrequencyGHz = 0.4
	singlecell.SetAllNodeProperty("BS", "TxPowerDBm", 22.0)
	for _, rxid := range rxids {
		metric := wsystem.EvaluteLinkMetric(&singlecell, &hatamodel, rxid, myfunc)
		RxMetrics1800[rxid] = metric
	}
	vlib.SaveStructure(RxMetrics1800, "metric1800MHz.json", true)

	// SINR := CalculateSINR(RxMetrics1800)

	var SINR vlib.VectorF
	// log.Println("Total Freqs", SINR)

	DumpMap2CSV("table400.dat", RxMetrics400)
	DumpMap2CSV("table1800.dat", RxMetrics1800)

	matlab.Close()

	matlab = vlib.NewMatlab("sinrVal.m")
	legendstring := ""
	for _, sinr := range SINR {

		str := fmt.Sprintf("sinr%d", int(wsystem.FrequencyGHz*1000))
		// str = strings.Replace(str, ".", "", -1)
		matlab.Export(str, sinr)
		matlab.Command("cdfplot(" + str + ");hold all;")
		legendstring += str + " "

	}
	matlab.Export("TxPower", singlecell.GetNodeType("BS").TxPowerDBm)
	matlab.Export("AntennaGainDb", defaultAAS.GainDb)
	matlab.Command(fmt.Sprintf("legend %v", legendstring))
	matlab.Close()
	fmt.Println("\n")
}

/// Calculate Pathloss

func DeployLayer1(system *deployment.DropSystem) {
	setting := system.GetSetting()

	if setting == nil {
		setting = deployment.NewDropSetting()

		GENERATE := true
		if GENERATE {
			// AreaRadius := CellRadius
			/// Should come from API
			// setting.SetCoverage(deployment.CircularCoverage(AreaRadius))

			/// NodeType should come from API calls
			newnodetype := deployment.NodeType{Name: "BS", Hmin: 30.0, Hmax: 30.0, Count: nCells * nSectors}
			newnodetype.Mode = deployment.TransmitOnly
			setting.AddNodeType(newnodetype)

			/// NodeType should come from API calls
			newnodetype = deployment.NodeType{Name: "UE", Hmin: 1.1, Hmax: 1.1, Count: nUEPerCell * nCells}
			newnodetype.Mode = deployment.ReceiveOnly
			setting.AddNodeType(newnodetype)

			vlib.SaveStructure(setting, "depSettings.json", true)

		} else {
			vlib.LoadStructure("depSettings.json", setting)
			fmt.Printf("\n %#v", setting.NodeTypes)
		}
		system.SetSetting(setting)

	}

	system.Init()

	// Workaround else should come from API calls or Databases
	bslocations := LoadBSLocations(system)
	system.SetAllNodeLocation("BS", vlib.Location3DtoVecC(bslocations))

	// Workaround else should come from API calls or Databases
	uelocations := LoadUELocations(system)
	system.SetAllNodeLocation("UE", uelocations)

	bsids := system.GetNodeIDs("BS")
	CreateAntennasForNetwork(system, bsids)

	vlib.SaveStructure(templateAAS, "antennaArray.json", true)
	vlib.SaveStructure(system, "dep.json", true)

}
func LoadBSLocations(system *deployment.DropSystem) []vlib.Location3D {
	/// Drop BS Nodes
	bslocations := make([]vlib.Location3D, system.NodeCount("BS"))

	clocations := deployment.HexGrid(nCells, vlib.FromCmplx(deployment.ORIGIN), CellRadius, 30)
	/// three nodes with single cell centere
	for i := 0; i < nCells; i++ {
		for k := 0; k < nSectors; k++ {
			bslocations[i*nSectors+k] = clocations[i]
		}
	}

	return bslocations
}
func LoadUELocations(system *deployment.DropSystem) vlib.VectorC {

	var uelocations vlib.VectorC
	hexCenters := deployment.HexGrid(nCells, vlib.FromCmplx(deployment.ORIGIN), CellRadius, 30)
	for indx, bsloc := range hexCenters {
		log.Printf("Deployed for cell %d ", indx)
		ulocation := deployment.HexRandU(bsloc.Cmplx(), CellRadius, nUEPerCell, 30)
		uelocations = append(uelocations, ulocation...)
	}

	return uelocations

}

func myfunc(nodeID int) antenna.SettingAAS {
	// atype := singlecell.Nodes[txnodeID]
	/// all nodeid same antenna
	obj, ok := templateAAS[nodeID]
	if !ok {
		log.Printf("No antenna created !! for %d ", nodeID)
		return defaultAAS
	} else {

		// fmt.Printf("\nNode %d , sector %v", nodeID, vlib.ModInt(nodeID, 3))
		return *obj
	}
}

func CreateAntennasForNetwork(system *deployment.DropSystem, bsids vlib.VectorI) {
	templateAAS = make(map[int]*antenna.SettingAAS)
	// sectorBW := 360.0 / float64(nSectors)
	for _, i := range bsids {
		temp := antenna.NewAAS()
		*temp = defaultAAS
		// temp.Set(str)
		// temp.Set(defaultAAS.Get())
		templateAAS[i] = temp
		templateAAS[i].FreqHz = CarriersGHz[0] * 1.e9
		// templateAAS[i].HBeamWidth = 65
		templateAAS[i].HTiltAngle = secangles[vlib.ModInt(i, 3)]
		if nSectors == 1 {
			templateAAS[i].Omni = true
		} else {
			templateAAS[i].Omni = false
		}
		templateAAS[i].CreateElements(system.Nodes[bsids[i]].Location)

		hgain := vlib.NewVectorF(360)
		cnt := 0
		cmd := `delta=pi/180;
		phaseangle=0:delta:2*pi-delta;`
		matlab.Command(cmd)
		for d := 0; d < 360; d++ {
			hgain[cnt] = templateAAS[i].ElementDirectionHGain(float64(d))
			cnt++
		}

		matlab.Export("gain"+strconv.Itoa(i), hgain)

		cmd = fmt.Sprintf("polar(phaseangle,gain%d);hold all", i)
		matlab.Command(cmd)
	}
}

func DumpMap2CSV(fname string, arg interface{}) {
	if reflect.TypeOf(arg).Kind() != reflect.Map {
		log.Println("Unable to Dump: Not Map interface")
		return
	}

	arrayData := reflect.ValueOf(arg)

	w, fer := os.Create(fname)
	if fer != nil {
		log.Print("Error Creating CSV file ", fer)
	}
	cwr := csv.NewWriter(w)
	// var record [4]string

	cwr.Comma = '\t'
	// w.WriteString("%NodeID\tFreqHz\tX\tY\tSINR\n")

	// once := false
	// counter := 0

	// for _, metric := range arrayData.Elem() {
	// count := arrayData.Len()
	// for i := 0; i < count; i++ {
	mapkeys := arrayData.MapKeys()
	once := true
	for _, key := range mapkeys {
		metric := arrayData.MapIndex(key).Interface()

		if once {
			headers, _ := vlib.Struct2Header(metric)
			w.WriteString("% " + strings.Join(headers, "\t") + "\n")
			once = false
		}
		// fmt.Printf("\nkey is %v ", key.Interface())
		// fmt.Printf("\nMETRIC is %v ", metric.Interface())
		// if counter < 10 {
		data, _ := vlib.Struct2Strings(metric)
		// }
		// counter++

		// temp.AppendAtEnd(metric[f].BestRSRP - (metric[f].N0))
		// SINR.AppendAtEnd(metric.BestSINR)
		// loc := singlecell.Nodes[metric.RxNodeID].Location
		// record := strings.Split(fmt.Sprintf("%d\t%f\t%f\t%f\t%f", metric.RxNodeID, metric.FreqInGHz, loc.X, loc.Y, metric.BestSINR), "\t")
		// fmt.Print(data)
		cwr.Write(data)
		// if counter < 10 {
		// 	fmt.Printf("\nrxid=%d indx %d Freq %f Value %v, %f", rxid, f, metric[f].FreqInGHz, metric[f].BestSINR, SINR[metric[f].FreqInGHz])
		// }
	} // }

	cwr.Flush()
	w.Close()
}
