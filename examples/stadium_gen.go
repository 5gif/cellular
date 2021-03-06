package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/grd/statistics"
	"github.com/wiless/cellular/antenna"
	"github.com/wiless/cellular/deployment"
	"github.com/wiless/cellular/pathloss"
	"github.com/wiless/vlib"

	"time"
)

var matlab *vlib.Matlab

var defaultAntenna *antenna.SettingAAS
var systemAntennas map[int]antenna.SettingAAS

// Dimension
// Outer Diameter : 283.01887m = 141.50944
// Inner Diameter : 174.5283m = 87.26415
type LinkInfo struct {
	RxID              int
	NodeTypes         []string
	LinkGain          vlib.VectorF
	LinkGainNode      vlib.VectorI
	InterferenceLinks vlib.VectorF
}

var angles vlib.VectorF = vlib.VectorF{45, -45, -135, -45}

func init() {
	CreateDefaultAntenna()
	matlab = vlib.NewMatlab("deployment")
	matlab.Silent = true
	matlab.Json = true
	rand.Seed(time.Now().Unix())
	systemAntennas = make(map[int]antenna.SettingAAS)
	vlib.LoadMapStructure("antennas.json", systemAntennas)

}

func main() {
	// fmt.Printf("The sample mean is %g", mean)

	var singlecell deployment.DropSystem

	// modelsett:=pathloss.NewModelSettingi()
	var model pathloss.PathLossModel
	model.ModelSetting.SetDefault()
	model.ModelSetting.Param[0] = 2
	SingleCellDeploy(&singlecell)

	/// Save deployment
	vlib.SaveStructure(&singlecell, "stadium.json", true)

	// fb, err := singlecell.MarshalJSON()
	// fmt.Println(err, "\n", string(fb))
	// fmt.Printf("\nOLDSYSTEM = %v \n\n", system)
	// dbytes, err := ioutil.ReadFile("system.json")
	// if err != nil {
	// 	log.Panicln("Unable to Marshal ", err)
	// }
	// var newsystem deployment.DropSystem
	// jerr := json.Unmarshal(dbytes, &newsystem)
	// if jerr != nil {
	// 	log.Panicln("Unable to unmarshal ", jerr)
	// } else {

	// 	fmt.Printf("\n\nNEWSYSTEM = %v", newsystem)
	// }

	// fmt.Println("\n======")
	///

	/// Generate Antennas for every Transmit Node
	/// If

	// templateAAS = antenna.NewAAS()
	// templateAAS.SetDefault()
	// templateAAS.N = 1
	// templateAAS.BeamTilt = 0
	// templateAAS.HTiltAngle = 0
	// templateAAS.VTiltAngle = 0
	// templateAAS.DisableBeamTit = true
	// templateAAS.Omni = false
	// var systemAntennas map[int]antenna.SettingAAS
	// systemAntennas = make(map[int]antenna.SettingAAS)
	// systemAntennas[0] = *templateAAS
	// templateAAS.Omni = true
	// systemAntennas[1] = *templateAAS

	// vlib.SaveMapStructure(systemAntennas, "antennas.json", true)

	ueLinkInfo := CalculatePathLoss(&singlecell, &model)
	rssi := vlib.NewVectorF(len(ueLinkInfo))
	for indx, val := range ueLinkInfo {

		temp := vlib.InvDbF(val.LinkGain)
		MaxSignal := vlib.Max(temp)

		TotalInterference := (vlib.Sum(temp) - MaxSignal) + vlib.Sum(vlib.InvDbF(val.InterferenceLinks))
		SIR := MaxSignal / TotalInterference
		rssi[indx] = vlib.Db(SIR)
	}
	// matlab.Export("rssi", rssi)

	singlecell.SetAllNodeProperty("UE", "AntennaType", 0)
	singlecell.SetAllNodeProperty("PICO", "AntennaType", 1)                /// Set All Pico to use antenna Type 0
	singlecell.SetAllNodeProperty("UE", "FreqGHz", vlib.VectorF{2.1, 3.5}) /// Set All Pico to use antenna Type 0

	vlib.SaveStructure(&singlecell, "stadiumOut.json", true)
	matlab.Export("SIR", rssi)
	matlab.ExportStruct("LinkInfo", ueLinkInfo)

	// matlab.ExportStruct("nodeTypes", singlecell.GetTxNodeNames())
	// log.Println("Exporting Node :")
	// matlab.ExportStruct("nodeinfo", singlecell.Nodes)
	// matlab.ExportStruct("nodeinfo1", *singlecell.Nodes[1])
	// for i := 0; i < len(singlecell.Nodes); i++ {
	// 	matlab.ExportStruct("Nodes"+strconv.Itoa(i), *singlecell.Nodes[i])
	// }

	matlab.Close()
	fmt.Println("\n")
}

/// Calculate Pathloss

func CalculatePathLoss(singlecell *deployment.DropSystem, model *pathloss.PathLossModel) []LinkInfo {

	txNodeNames := singlecell.GetTxNodeNames()
	// txNodeNames = []string{"BS"} /// do only for BS

	rxNodeNames := singlecell.GetRxNodeNames()
	log.Println(txNodeNames, rxNodeNames)

	// rxlocs := singlecell.Locations("UE")
	rxlocs3D := singlecell.Locations3D(rxNodeNames[0])
	RxLinkInfo := make([]LinkInfo, len(rxlocs3D))

	/// Generate Shadow Grid

	fmt.Printf("SETTING %s", singlecell.CoverageRegion.Celltype)

	// shwGrid := vlib.NewMatrixF(rows, cols)
	// for i := 0; i < len(rxlocs3D); i++ {
	// 	rxlocation := rxlocs3D[i]
	// 	var info LinkInfo
	// 	info.RxID = i
	// }

	var pathLossPerRxNode map[int]vlib.VectorF
	pathLossPerRxNode = make(map[int]vlib.VectorF)
	log.Println(pathLossPerRxNode)
	for i := 0; i < len(rxlocs3D); i++ {
		rxlocation := rxlocs3D[i]
		var info LinkInfo
		info.RxID = i

		func(rxlocation vlib.Location3D, txNodeNames []string) {
			info.NodeTypes = make([]string, len(txNodeNames))
			info.LinkGain = vlib.NewVectorF(len(txNodeNames))
			info.LinkGainNode = vlib.NewVectorI(len(txNodeNames))
			info.InterferenceLinks = vlib.NewVectorF(len(txNodeNames))
			for indx, name := range txNodeNames {
				txlocs := singlecell.Locations(name)
				txLocs3D := singlecell.Locations3D(name)

				allpathlossPerTxType := vlib.NewVectorF((txlocs.Size()))

				info.NodeTypes[indx] = name
				N := txlocs.Size()
				txnids := singlecell.GetNodeIDs(name)
				for k := 0; k < N; k++ {
					node := singlecell.Nodes[txnids[k]]
					aid := node.AntennaType
					// antenna:= systemAntennas[txn]
					// angle := float64((k) * 360 / N)

					antenna := systemAntennas[aid]
					antenna.HTiltAngle, antenna.VTiltAngle = node.Orientation[0], node.Orientation[1]
					fmt.Printf("\n For Rx(%d) %s [%d]. antenna = %v", info.RxID, name, txnids[k], antenna)
					antenna.CreateElements(txLocs3D[k])
					distance, _, _ := vlib.RelativeGeo(txLocs3D[k], rxlocation)
					lossDb := model.LossInDb(distance)
					aasgain, _, _ := antenna.AASGain(rxlocation) /// linear scale
					totalGainDb := vlib.Db(aasgain) - lossDb
					allpathlossPerTxType[k] = totalGainDb

					// fmt.Printf("\n Distance %f : loss %f dB", distance, lossDb)
					// matlab.Export(matstr, data)
				}
				data := statistics.Float64(allpathlossPerTxType)
				info.LinkGain[indx], info.LinkGainNode[indx] = statistics.Max(&data) // dB
				info.InterferenceLinks[indx] = vlib.Db(vlib.Sum(vlib.InvDbF(allpathlossPerTxType)) - vlib.InvDb(info.LinkGain[indx]))

			}

		}(rxlocation, txNodeNames)
		RxLinkInfo[i] = info
		// fmt.Printf("\n Info[%d] : %#v", i, info)
	}
	return RxLinkInfo
}

func SingleCellDeploy(system *deployment.DropSystem) {

	setting := deployment.NewDropSetting()
	temp := deployment.NewDropSetting()
	temp.SetDefaults()

	CellRadius := 141.50944
	AreaRadius := CellRadius
	setting.SetCoverage(deployment.CircularCoverage(AreaRadius))

	StadiumInnerRadius := 87.26415
	StadiumOuterRadius := 141.50944

	/// Total PICO nodes required
	// deltaOffset := 20.0 // 20m
	OuterArea := math.Pi * StadiumOuterRadius * StadiumOuterRadius
	InnerArea := math.Pi * StadiumInnerRadius * StadiumInnerRadius
	MinDistance := 20.0 / 2
	PicoCount := int(math.Ceil((OuterArea - InnerArea) / (math.Pi * MinDistance * MinDistance)))
	PicoCount = 2
	log.Println("Total Nodes Per Ring", PicoCount)

	setting.AddNodeType(deployment.NodeType{Name: "UE", Hmin: 1.0, Hmax: 10.0, Count: 10})
	setting.AddNodeType(deployment.NodeType{Name: "PICO", Hmin: 20.0, Hmax: 25.0, Count: PicoCount})
	/// You can save the settings of this deployment by uncommenting this line

	setting.SetTxNodeNames("PICO")
	setting.SetRxNodeNames("UE")
	system.SetSetting(setting)
	system.Init()

	vlib.SaveStructure(setting, "nodetype.txt", true)

	// jerr, jbytes := system.MarshalJSON()
	// jbytes, jerr := json.Marshal(system)
	// fmt.Println("===============")
	// fmt.Print(jerr, jbytes)
	// fmt.Println("===============")
	// jbytes, jerr = json.Marshal(setting)
	// fmt.Println("===============")
	// fmt.Print(jerr, string(jbytes))
	// fmt.Println("===============")

	/// Drop UE Nodes
	{
		locations := deployment.AnnularRingPoints(deployment.ORIGIN, StadiumInnerRadius, StadiumOuterRadius, system.NodeCount("UE"))
		system.SetAllNodeLocation("UE", locations)
	}

	/// Drop PICO Nodes
	{
		var PICOlocations vlib.VectorC
		random := true
		if !random {
			// offset := 10
			radius := StadiumInnerRadius + 10.0
			for i := 0; i < PicoCount; {
				count := int(math.Floor(2.0 * math.Pi * radius / 20.0))

				locations := deployment.AnnularRingEqPoints(deployment.ORIGIN, radius, count)
				PICOlocations.AppendAtEnd(locations...)
				i += count
				// fmt.Printf("\n Total %d , Current %d : %v", i, count, PICOlocations)

				radius += 20.0
			}

		} else {
			PICOlocations = deployment.AnnularRingPoints(deployment.ORIGIN, StadiumInnerRadius, StadiumOuterRadius, PicoCount)
		}
		system.SetAllNodeLocation("PICO", PICOlocations)

	}

	system.SetAllNodeProperty("UE", "AntennaType", 0)
	system.SetAllNodeProperty("PICO", "AntennaType", 1) /// Set All Pico to use antenna Type 0
	// fvec := vlib.VectorF{2.1, 1.8, 3.4}
	// system.SetAllNodeProperty("PICO", "FreqGHz", fvec) /// Set All Pico to use antenna Type 0

	matlab.Export("ue", system.Locations("UE"))
	matlab.Export("pico", system.Locations("PICO"))

	plotcmd := `hold off;
	plot(real(ue),imag(ue),'.');
	hold all;
	plot(real(pico),imag(pico),'k*');
	grid on;`

	matlab.Command(plotcmd)
	// 	looptxt := `for k=1:length(bs)
	// text(real(bs(k)),imag(bs(k)),'BS')
	// end`
	// 	matlab.Q(looptxt)

	/// Plot scatter
	scattercmd := `figure;C=colormap;
	deltaRssi=80/64;
	deltasize=80/14;
	S=floor((SIR+110)/deltasize);
cindx=floor(SIR/deltaRssi);
scatter3(real(ue),imag(ue),SIR,64,cindx,'filled');
colorbar;
view(2)
`
	matlab.Q(scattercmd)

}

func CreateDefaultAntenna() {
	defaultAntenna = antenna.NewAAS()
	defaultAntenna.SetDefault()
	defaultAntenna.N = 1
	defaultAntenna.BeamTilt = 0
	defaultAntenna.HTiltAngle = 0
	defaultAntenna.VTiltAngle = 0
	defaultAntenna.DisableBeamTit = true
	defaultAntenna.Omni = false
}
