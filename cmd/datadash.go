package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	termutil "github.com/andrew-d/go-termutil"
	ui "github.com/gizak/termui"
	w "github.com/gizak/termui/widgets"
	"github.com/montanaflynn/stats"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	linegraph "github.com/keithknott26/datadash"
	plot "github.com/keithknott26/datadash"
)

var (
	app       = kingpin.New("lineplot", "A CCS Graphing Application")
	debug     = app.Flag("debug", "Enable Debug Mode").Bool()
	delimiter = app.Flag("delimiter", "Record Delimiter").Default("\t").String()
	lineMode  = app.Flag("line-mode", "Line Mode: Dot, Braille").Short('l').Default("braille").String()
	graphMode = app.Flag("graph-mode", "Graph Type: line, scatter").Short('g').Default("line").String()
	labelMode = app.Flag("label-mode", "X-Axis Labels: first, time").Short('m').Default("first").String()
	hScale    = app.Flag("horizontal-scale", "Horizontal Scale: (1,2,3,...)").Short('h').Default("1").Int()
	inputFile = app.Arg("input file", "A file containing data in rows delimited by delimiter 'd'").File()

	grid     *ui.Grid
	plot1    *w.Plot
	plot1buf *linegraph.Buffer
	plot2    *w.Plot
	plot2buf *linegraph.Buffer
	plot3    *w.Plot
	plot3buf *linegraph.Buffer
	plot4    *w.Plot
	plot4buf *linegraph.Buffer
	par1     *w.Paragraph
	par2     *w.Paragraph
	par3     *w.Paragraph
	par4     *w.Paragraph

	//data and labels
	lineChartData       = make([]float64, 0, 10)
	lineChartDataLabels = make([]string, 0, 10)
	labels              = make([]string, 0, 10)
	graphs              = 1
	termWidth           = 1
	termHeight          = 1
	drawInterval        = 200 * time.Millisecond
	interrupt           = false
	resume              = false
	//updateInterval      = time.Second
	updateInterval = 225 * time.Millisecond
	//for pausing the graph
	//define colors
	//0 black
	//1 red
	//2 green
	//3 yellow
	//4 blue
	//5 magenta
	//6 cyan
	//7 white
	//8 grey
	//9 peach
	//10 light green
	//11 light yellow
	//12 purple
	//13 light pink
	//14 light cyan
	//15 white
	//16 black
	//17 dark blue
	//18 dark blue
	//19 dark blue
	//20 med blue
	Foreground  = 7
	Background  = -1
	BorderLabel = 7
	BorderLine  = 6
	GraphBorder = 6
	GraphLines  = 6
	GraphAxes   = 8
	GraphTitles = 2
	ParBorder   = 6
	ParTitle    = 2
	ParText     = 3
	SwapMem     = 11
	ProcCursor  = 4
	Sparkline   = 4
	DiskBar     = 7
	TempLow     = 2
	TempHigh    = 1
	PlotLines   = 3
	LineTwo     = 6
	LineThree   = 1
	LineFour    = 5
	LineFive    = 7
	LinePaused  = 1
)

func update(record []string) [][]float64 {
	n := 220
	data := make([][]float64, 2)
	data[0] = make([]float64, n)
	data[1] = make([]float64, n)
	for i := 0; i < n; i++ {
		data[0][i] = 1 + math.Sin(float64(i)/5)
		data[1][i] = 1 + math.Cos(float64(i)/5)
	}
	return data
}

func initWidgets(records []string) {
	//plot 1
	plot1 = w.NewPlot()
	plot1.Title = "braille-mode Line Chart"
	plot1.AxesColor = ui.Color(GraphAxes)
	plot1.HorizontalScale = *hScale / 1
	plot1.DrawDirection = w.DrawRight
	plot1.BorderStyle.Fg = ui.Color(BorderLine)
	plot1buf = plot.NewBuffer(1440)

	//plot 2
	plot2 = w.NewPlot()
	plot2.Title = "braille-mode Line Chart"
	plot2.AxesColor = ui.Color(GraphAxes)
	plot2.LineColors[0] = ui.Color(LineThree)
	plot2.HorizontalScale = *hScale / 1
	plot2.DrawDirection = w.DrawRight
	plot2.BorderStyle.Fg = ui.Color(BorderLine)
	plot2buf = plot.NewBuffer(1440)

	//plot3
	plot3 = w.NewPlot()
	plot3.Title = "braille-mode Line Chart"
	plot3.AxesColor = ui.Color(GraphAxes)
	plot3.LineColors[0] = ui.Color(LineThree)
	plot3.HorizontalScale = *hScale / 1
	plot3.DrawDirection = w.DrawRight
	plot3.BorderStyle.Fg = ui.Color(BorderLine)
	plot3buf = plot.NewBuffer(1440)

	//plot4
	plot4 = w.NewPlot()
	plot4.Title = "braille-mode Line Chart"
	plot4.AxesColor = ui.Color(GraphAxes)
	plot4.LineColors[0] = ui.Color(LineThree)
	plot4.HorizontalScale = *hScale / 1
	plot4.DrawDirection = w.DrawRight
	plot4.BorderStyle.Fg = ui.Color(BorderLine)
	plot4buf = plot.NewBuffer(1440)
	if *lineMode == "dot" {
		//plot1.DotRune = '.'
		plot1.Marker = w.MarkerDot
		plot2.Marker = w.MarkerDot
		plot3.Marker = w.MarkerDot
		plot4.Marker = w.MarkerDot
	} else {
		plot1.Marker = w.MarkerBraille
		plot2.Marker = w.MarkerBraille
		plot3.Marker = w.MarkerBraille
		plot4.Marker = w.MarkerBraille
	}
	if *graphMode == "scatter" {
		plot1.Type = w.ScatterPlot
		plot2.Type = w.ScatterPlot
		plot3.Type = w.ScatterPlot
		plot4.Type = w.ScatterPlot
	}

	par1 = w.NewParagraph()
	par1.BorderStyle.Fg = ui.Color(ParBorder)
	par2 = w.NewParagraph()
	par2.BorderStyle.Fg = ui.Color(ParBorder)
	par3 = w.NewParagraph()
	par3.BorderStyle.Fg = ui.Color(ParBorder)
	par4 = w.NewParagraph()
	par4.BorderStyle.Fg = ui.Color(ParBorder)
}

func termuiColors() {
	ui.Theme.Default = ui.NewStyle(ui.Color(Foreground), ui.Color(Background))
	ui.Theme.Block.Title = ui.NewStyle(ui.Color(BorderLabel), ui.Color(Background))
	ui.Theme.Block.Border = ui.NewStyle(ui.Color(BorderLine), ui.Color(Background))
	ui.Theme.Block.Title.Fg = ui.Color(GraphTitles)
	//graph lines (not working)
	var standardColors = []ui.Color{
		ui.ColorGreen,
		ui.ColorGreen,
		ui.ColorYellow,
		ui.ColorBlue,
		ui.ColorMagenta,
		ui.ColorCyan,
		ui.ColorWhite,
	}
	ui.Theme.Plot.Lines = standardColors
	ui.Theme.Plot.Axes = ui.ColorGreen
	//paragraph definitions
	ui.Theme.Paragraph.Text = ui.NewStyle(ui.Color(ParText), ui.Color(Background))

}
func setupGrid() {
	if *debug == false {
		//setup the layout
		//p4.SetRect(100, 5, 35, 10)
		grid = ui.NewGrid()
		termWidth, termHeight := ui.TerminalDimensions()
		grid.SetRect(0, 0, termWidth, termHeight)

		if graphs == 1 {
			plotRow1 := ui.NewRow(1.0/1,
				ui.NewCol(1.0/6, par1),
				ui.NewCol(5.0/6, plot1),
			)
			grid.Set(plotRow1)
		} else if graphs == 2 {
			plotRow1 := ui.NewRow(1.0/2,
				ui.NewCol(1.0/6, par1),
				ui.NewCol(5.0/6, plot1),
			)
			plotRow2 := ui.NewRow(1.0/2,
				ui.NewCol(1.0/6, par2),
				ui.NewCol(5.0/6, plot2),
			)
			grid.Set(plotRow1, plotRow2)
		} else if graphs == 3 {
			plotRow1 := ui.NewRow(1.0/3,
				ui.NewCol(1.0/6, par1),
				ui.NewCol(5.0/6, plot1),
			)
			plotRow2 := ui.NewRow(1.0/3,
				ui.NewCol(1.0/6, par2),
				ui.NewCol(5.0/6, plot2),
			)
			plotRow3 := ui.NewRow(1.0/3,
				ui.NewCol(1.0/6, par3),
				ui.NewCol(5.0/6, plot3),
			)
			grid.Set(plotRow1, plotRow2, plotRow3)
		} else if graphs == 4 {
			plotRow1 := ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, par1),
				ui.NewCol(5.0/6, plot1),
			)
			plotRow2 := ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, par2),
				ui.NewCol(5.0/6, plot2),
			)
			plotRow3 := ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, par3),
				ui.NewCol(5.0/6, plot3),
			)
			plotRow4 := ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, par4),
				ui.NewCol(5.0/6, plot4),
			)
			grid.Set(plotRow1, plotRow2, plotRow3, plotRow4)
		} else {
			log.Fatalf("Error: Too many columns, Max columns to display is 4.")
		}
		ui.Render(grid)
	}
}

func addLineChartData(record []string, label string, lineChartData [][]float64, lineChartDataLabels [][]string, labels []string) {
	//for i := 0; i < len(record); i++ {
	for i, x := range record {
		if i == 0 {
			if *debug {
				fmt.Println("Record[i]:", record[i])
				fmt.Println("i:", i)
				fmt.Println("x:", x)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			lineChartData[i] = append(lineChartData[i], val)
			lineChartDataLabels[i] = append(lineChartDataLabels[i], strings.TrimSpace(label))
			//label = data label (from row 1 or timestamp)
			//x = record (from delimited row)
			plot1buf.Add(val, label)
			//calculate max value of slice for axes
			max, _ := stats.Max(lineChartData[i])
			plot1.MaxVal = max
			plot1.Title = " [" + labels[i+1] + "] - 'p' Pause, <- Slower , -> Faster "
			updateParagraph1(par1, labels[i+1], label, lineChartData[i])
			plot1.LineColors[0] = ui.ColorGreen //ui.Color(PlotLines)

			if val/max > float64(.50) {
				plot1.LineColors[0] = ui.ColorYellow //ui.Color(PlotLines)
			}
			if val/max > float64(.75) {
				plot1.LineColors[0] = ui.ColorRed
			}
			if val/max > float64(.90) {
				plot1.LineColors[0] = ui.ColorMagenta
			}
		}
		if i == 1 {
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			lineChartData[i] = append(lineChartData[i], val)
			lineChartDataLabels[i] = append(lineChartDataLabels[i], strings.TrimSpace(label))
			//label = data label (from row 1 or timestamp)
			//x = record (from delimited row)
			plot2buf.Add(val, label)
			//plot2.LineColors[0] = ui.ColorCyan
			//plot2.LineColors[0] = ui.ColorYellow
			//calculate max value of slice for axes
			max, _ := stats.Max(lineChartData[i])
			plot2.MaxVal = max
			plot2.Title = " [" + labels[i+1] + "] - 'p' Pause, <- Slower , -> Faster "
			updateParagraph2(par2, labels[i+1], label, lineChartData[i])
		}
		if i == 2 {
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			lineChartData[i] = append(lineChartData[i], val)
			lineChartDataLabels[i] = append(lineChartDataLabels[i], strings.TrimSpace(label))
			//label = data label (from row 1 or timestamp)
			//x = record (from delimited row)
			plot3buf.Add(val, label)
			//plot2.LineColors[0] = ui.ColorCyan
			//plot2.LineColors[0] = ui.ColorYellow
			//calculate max value of slice for axes
			max, _ := stats.Max(lineChartData[i])
			plot3.MaxVal = max
			plot3.Title = " [" + labels[i+1] + "] - 'p' Pause, <- Slower , -> Faster "
			updateParagraph3(par3, labels[i+1], label, lineChartData[i])
		}
		if i == 3 {
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			lineChartData[i] = append(lineChartData[i], val)
			lineChartDataLabels[i] = append(lineChartDataLabels[i], strings.TrimSpace(label))
			//label = data label (from row 1 or timestamp)
			//x = record (from delimited row)
			plot4buf.Add(val, label)
			//plot2.LineColors[0] = ui.ColorCyan
			//plot2.LineColors[0] = ui.ColorYellow
			//calculate max value of slice for axes
			max, _ := stats.Max(lineChartData[i])
			plot4.MaxVal = max
			plot4.Title = " [" + labels[i+1] + "] - 'p' Pause, <- Slower , -> Faster "
			updateParagraph4(par4, labels[i+1], label, lineChartData[i])
		}

		if *debug {
			fmt.Println("Data[i] Array:", lineChartData[i])
			fmt.Println("Column Label:", label)
			fmt.Println("Column Record:", x)
		}
		//plot1.MaxVal, _ = ui.GetMaxFloat64FromSlice(plot1buf.DataContainer)
	}
	if *debug {
		//fmt.Println("For Loop lineChartData:", lineChartData)
		//fmt.Println("For Loop lineChartDataLabels:", lineChartDataLabels)
	}
	//data := plot1buf.Buffer()
	//var datalabels1 []string
	var data1 []float64
	var data2 []float64
	var data3 []float64
	var data4 []float64
	if *debug == false {
		termWidth, termHeight := ui.TerminalDimensions()
		//set termWidth to actual width of linechart 83% of screen
		termWidth = int((float64(termWidth)*float64(0.83) - 3))
		data1 = plot1buf.Last(termHeight, termWidth)
		data2 = plot2buf.Last(termHeight, termWidth)
		data3 = plot3buf.Last(termHeight, termWidth)
		data4 = plot4buf.Last(termHeight, termWidth)
	} else {
		data1 = plot1buf.Last(100, 100)
		data2 = plot2buf.Last(100, 100)
		data3 = plot3buf.Last(100, 100)
		data4 = plot4buf.Last(100, 100)
	}

	plot1.Data = make([][]float64, 1)
	plot2.Data = make([][]float64, 1)
	plot3.Data = make([][]float64, 1)
	plot4.Data = make([][]float64, 1)
	//plot1.Data[0] = []float64{1}
	//plot1.LineColors[0] = ui.Color(LineOne)
	//plot2.LineColors[0] = ui.Color(LineThree)

	//set the labels
	plot1.DataLabels = plot1buf.LastLabels(termHeight, termWidth)
	plot2.DataLabels = plot2buf.LastLabels(termHeight, termWidth)
	plot3.DataLabels = plot3buf.LastLabels(termHeight, termWidth)
	plot4.DataLabels = plot4buf.LastLabels(termHeight, termWidth)

	for _, x := range data1 {
		plot1.Data[0] = append(plot1.Data[0], x)
	}
	for _, x := range data2 {
		plot2.Data[0] = append(plot2.Data[0], x)
	}
	for _, x := range data3 {
		plot3.Data[0] = append(plot3.Data[0], x)
	}
	for _, x := range data4 {
		plot4.Data[0] = append(plot4.Data[0], x)
	}
	if *debug {
		//fmt.Println("Buffer:", plot1buf.Buffer())
		fmt.Println("Data plot1:", plot1.Data[0])
		fmt.Println("Data plot2:", plot2.Data[0])
		fmt.Println("Data plot3:", plot3.Data[0])
		fmt.Println("Data plot4:", plot4.Data[0])
		//plot1.DataLabels = lineChartDataLabels[1]
		//fmt.Println("Data plot1 len:", len(data))
		//fmt.Println("Data plot1 cap:", cap(data))
		//fmt.Println("Data plot1buf len:", len(plot1buf.Data[0]))
		//fmt.Println("Data plot1buf cap:", cap(plot1buf.Data[0]))
		//plot1.Data[0] = data
		//plot1.Data[0] = append(plot1buf.Data, data)
		//plot1.Data[1] = lineChartData[1]
		//plot1.Data[2] = lineChartData[2]
		//plot1.Data[3] = lineChartData[3]
		//plot1.Data[0] = plot1buf.Data[0]
		//plot1.Data[1] = plot1buf.Data[1]
		//plot1.Data[2] = plot1buf.Data[2]
		//plot1.Data[3] = plot1buf.Data[3]
		// You can change the color values in your plot widget by checking if incoming
		// data meets a certain threshold and modifying the corresponding entry in the LineColors field.
		//plot1.LineColors[0] = ui.Color(LineOne)
		//plot1.LineColors[1] = ui.Color(LineTwo)
		//plot1.LineColors[2] = ui.Color(LineThree)
		//plot1.LineColors[3] = ui.Color(LineFour)
		//plot1.LineColors[4] = ui.Color(LineFive)
	}

	if *debug {
		//fmt.Println("PlotWidget Data[0]: ", plot1buf.Data[0])
		//fmt.Println("PlotWidget Data[1]: ", plot1buf.Data[1])
		//fmt.Println("PlotWidget Data[2]: ", plot1buf.Data[2])
		//fmt.Println("PlotWidget Data[3]: ", plot1buf.Data[3])
		////fmt.Println("lineChartData:", lineChartData[0])
		//fmt.Println("sinData:", sinData)
		//fmt.Println("Plot 0 Data:", plot1.Data)
		//fmt.Println("plot1buf Data:", plot1buf.Data)
		////fmt.Println("lineChartDataLabels:", lineChartDataLabels[0])
	}

}

func updateParagraph1(par *w.Paragraph, header string, pointer string, records []float64) {
	par1.Text = prepareStats(header, pointer, records)
}

func updateParagraph2(par *w.Paragraph, header string, pointer string, records []float64) {
	par2.Text = prepareStats(header, pointer, records)
}
func updateParagraph3(par *w.Paragraph, header string, pointer string, records []float64) {
	par3.Text = prepareStats(header, pointer, records)
}
func updateParagraph4(par *w.Paragraph, header string, pointer string, records []float64) {
	par4.Text = prepareStats(header, pointer, records)
}

func prepareStats(header string, pointer string, records []float64) string {
	//calculate stat values
	pointervalue := records[len(records)-1]
	data := stats.LoadRawData(records)
	//fmt.Println("PREPARESTATS:DATA:", data)
	outliers, _ := stats.QuartileOutliers(data)
	median, _ := stats.Median(data)
	mean, _ := stats.Mean(data)
	min, _ := stats.Min(data)
	max, _ := stats.Max(data)
	//variance, _ := stats.Variance(data)
	//stddev, _ := stats.StandardDeviation(data)
	outlier := 0.00
	for _, i := range outliers.Extreme {
		outlier = i
	}
	//count records
	count := len(records)
	text := fmt.Sprintf("[%s](fg:blue,mod:bold)\nCount:       %d\nMin:         %.2f\nMean:        %.2f\nMedian:      %.2f\nMax:         %.2f\nOutliers:    %.2f\nTime:        [%s](fg:white)\nValue:       [%.2f](fg:white)",
		header, count, min, mean, median, max, outlier, pointer, pointervalue)

	return text
}

//For Tailing Log Files: https://github.com/hpcloud/tail
func main() {
	//setup vars for pause / resume
	reset := true
	slower := false
	faster := false

	// Parse args and assign values
	kingpin.Version("0.0.1")
	kingpin.MustParse(app.Parse(os.Args[1:]))
	if *debug {
		fmt.Printf("Running with: Delimiter: %s lineMode: %s labelMode: %s graphMode: %s\n", *delimiter, *lineMode, *labelMode, *graphMode)
	}
	//define the reader type (Stdin or File based)
	var reader *csv.Reader
	// read file in or Stdin
	if *inputFile != nil {
		reader = csv.NewReader(bufio.NewReader(*inputFile))
		//defer file.Close()
	} else if !termutil.Isatty(os.Stdin.Fd()) {
		reader = csv.NewReader(bufio.NewReader(os.Stdin))
	} else {
		return
	}
	reader.Comma = []rune(*delimiter)[0]

	//read the first line as labels
	labels, err := reader.Read()
	if err != nil {
		panic(err)
	}
	//read the second line as data
	records, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return
		}
		panic(err)
	}
	//calculate number of graphs (max 4)
	graphs = len(records) - 1

	//print data
	if *debug {
		fmt.Println("Records Array:", records)
		fmt.Println("Number of Graphs:", graphs)
		fmt.Println("Labels Array:", labels)
	}
	//Create lineChartDataLabel data structure
	lineChartDataLabels := make([][]string, len(labels))
	for i := 0; i < len(labels); i++ {
		lineChartDataLabels[i] = make([]string, 1)
	}
	//Create lineChartData data structure
	lineChartData := make([][]float64, len(records))
	for i := 0; i < len(records); i++ {
		lineChartData[i] = make([]float64, 1)
	}
	// read from Reader (Stdin or File) into a dataChan
	dataChan := make(chan []string, 10)
	go func() {
		for {
			if faster == true {
				time.Sleep(30 * time.Millisecond)
				//tickInterval = time.Duration(offset) * time.Millisecond
			}
			if reset == true {
				time.Sleep(70 * time.Millisecond)
			}
			if slower == true {
				time.Sleep(350 * time.Millisecond)
				//tickInterval = time.Duration(offset) * time.Millisecond
			}
			if interrupt == true {
				time.Sleep(10 * time.Second)
				interrupt = false
				//tickInterval = time.Duration(offset) * time.Millisecond
			}
			r, err := reader.Read()
			if err != nil {
				if err == io.EOF {
					return
				}
				panic(err)
			}
			dataChan <- r
		}
	}()
	////////////// END READ FROM STDIN OR FILE ///////////////////
	if *debug == false {
		if err := ui.Init(); err != nil {
			log.Fatalf("failed to initialize termui: %v", err)
		}
		defer ui.Close()
	}
	////////////// END INIT TERMUI  ///////////////////
	//theme colors
	termuiColors()
	//init the widgets
	initWidgets(records)

	updateLinechart := func(records []string) {
		label := records[0]
		record := records[1:]
		if *labelMode == "time" {
			//Use the time as a X-Axis labels
			now := time.Now()
			label = fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())
		} else {
			label = records[0]
			record = records[1:]
		}
		//populate lineChartData and lineChartDataLables
		addLineChartData(record, label, lineChartData, lineChartDataLabels, labels)
	}
	if *debug {
		//fmt.Println("lineChartData:", lineChartData[0])
		//fmt.Println("plot1.Data:", plot1.Data)
		//fmt.Println("lineChartDataLabels:", lineChartDataLabels)
	}

	if *debug {
		//fmt.Println("lineChartData:", lineChartData[0])
		//fmt.Println("plot1.Data:", plot1.Data)
		//fmt.Println("lineChartDataLabels:", lineChartDataLabels)
	}
	//pause := func() {
	//run = !run
	//if run {
	//	plot1.Title = "braille Line Chart"
	//} else {
	//	plot1.Title = "braille Line Chart (Paused)"
	//	plot1.LineColors[0] = ui.Color(LinePaused)
	//	plot2.LineColors[0] = ui.Color(LinePaused)
	//	time.Sleep(5 * time.Second)
	//}
	//ui.Render(plot1, plot2)
	//}

	//setup the grid
	setupGrid()
	uiEvents := ui.PollEvents()

	ticker := time.NewTicker(updateInterval).C
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			case "p":
				interrupt = true
			case "<Space>":
				reset = true
			case "<Right>":
				slower = false
				faster = true
				reset = false
			case "<Left>":
				slower = true
				faster = false
				reset = false
			case "<Resize>":
				payload := e.Payload.(ui.Resize)
				grid.SetRect(0, 0, payload.Width, payload.Height)
				ui.Clear()
				ui.Render(grid)
			}
		case <-ticker:
			//if run {
			if *debug == false {
				//ui.Clear()
				ui.Render(grid)
			}
			//}
		case record := <-dataChan:
			updateLinechart(record)
			//updateParagraph(labels)
		}
	}
}
