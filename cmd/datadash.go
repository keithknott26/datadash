package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	termutil "github.com/andrew-d/go-termutil"
	ui "github.com/gizak/termui"
	"github.com/montanaflynn/stats"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/draw"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/terminal/termbox"
	"github.com/mum4k/termdash/terminalapi"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/text"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	ring "github.com/keithknott26/datadash"
)

var (
	app       = kingpin.New("datadash", "A CCS Graphing Application")
	debug     = app.Flag("debug", "Enable Debug Mode").Bool()
	delimiter = app.Flag("delimiter", "Record Delimiter").Default("\t").String()
	lineMode  = app.Flag("line-mode", "Line Mode: Dot, Braille").Short('l').Default("braille").String()
	graphMode = app.Flag("graph-mode", "Graph Type: line, scatter").Short('g').Default("line").String()
	labelMode = app.Flag("label-mode", "X-Axis Labels: first, time").Short('m').Default("first").String()
	hScale    = app.Flag("horizontal-scale", "Horizontal Scale: (1,2,3,...)").Short('h').Default("1").Int()
	inputFile = app.Arg("input file", "A file containing data in rows delimited by delimiter 'd'").File()

	plot1buf *ring.Buffer
	plot2buf *ring.Buffer
	plot3buf *ring.Buffer
	plot4buf *ring.Buffer

	ctx context.Context
	//to be removed
	floatvalues = make([]float64, 1)
	dataChan    = make(chan []string, 1)
	//keep
	lineChartData       = make([][]float64, 0, 10)
	lineChartDataLabels = make([][]string, 0, 10)
	labels              = make([]string, 0, 10)
	graphs              = 1
	termWidth           = 1
	termHeight          = 1
	linchartWidth       = 1
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
	//165 Orange
	Foreground = 7
	Background = -1

	ParOneBorder     = 27
	ParOneTitle      = 82
	ParTwoBorder     = 27
	ParTwoTitle      = 13
	ParThreeBorder   = 27
	ParThreeTitle    = 45
	ParFourBorder    = 27
	ParFourTitle     = 9
	ParText          = 3
	ParPointer       = 8
	ParValue         = 15
	LineLow          = 2
	LineHigh         = 1
	GraphOneBorder   = 27
	GraphTwoBorder   = 27
	GraphThreeBorder = 27
	GraphFourBorder  = 27
	GraphAxes        = 8
	GraphXLabels     = 242
	GraphYLabels     = 15
	GraphTitles      = 2
	GraphLineOne     = 82
	GraphLineTwo     = 13
	GraphLineThree   = 45
	GraphLineFour    = 9
	GraphLinePaused  = 1
)

// redrawInterval is how often termdash redraws the screen.
const redrawInterval = 250 * time.Millisecond

func layout(ctx context.Context, t terminalapi.Terminal, labels []string) (*container.Container, error) {
	if len(labels)-1 == 1 {
		FirstRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParOneBorder)),
					container.PlaceWidget(newTextOne(ctx, labels[1])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[1]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphOneBorder)),
					container.PlaceWidget(newPlotOne(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		c, err := container.New(t, FirstRow...)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if len(labels)-1 == 2 {
		FirstRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParOneBorder)),
					container.PlaceWidget(newTextOne(ctx, labels[1])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[1]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphOneBorder)),
					container.PlaceWidget(newPlotOne(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		SecondRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParTwoBorder)),
					container.PlaceWidget(newTextTwo(ctx, labels[2])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[2]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphTwoBorder)),
					container.PlaceWidget(newPlotTwo(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		c, err := container.New(
			t,
			container.SplitHorizontal(
				container.Top(FirstRow...),
				container.Bottom(SecondRow...),
				container.SplitPercent(50),
			),
		)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if len(labels)-1 == 3 {
		FirstRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParOneBorder)),
					container.PlaceWidget(newTextOne(ctx, labels[1])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[1]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphOneBorder)),
					container.PlaceWidget(newPlotOne(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		SecondRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParTwoBorder)),
					container.PlaceWidget(newTextTwo(ctx, labels[2])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[2]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphTwoBorder)),
					container.PlaceWidget(newPlotTwo(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		TopHalf := []container.Option{
			container.SplitHorizontal(
				container.Top(FirstRow...),
				container.Bottom(SecondRow...),
				container.SplitPercent(50),
			),
		}
		ThirdRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParThreeBorder)),
					container.PlaceWidget(newTextThree(ctx, labels[3])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[3]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphThreeBorder)),
					container.PlaceWidget(newPlotThree(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		c, err := container.New(
			t,
			container.SplitHorizontal(
				container.Top(TopHalf...),
				container.Bottom(ThirdRow...),
				container.SplitPercent(66),
			),
		)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if len(labels)-1 == 4 {
		FirstRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParOneBorder)),
					container.PlaceWidget(newTextOne(ctx, labels[1])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[1]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphOneBorder)),
					container.PlaceWidget(newPlotOne(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		SecondRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParTwoBorder)),
					container.PlaceWidget(newTextTwo(ctx, labels[2])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[2]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphTwoBorder)),
					container.PlaceWidget(newPlotTwo(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		TopHalf := []container.Option{
			container.SplitHorizontal(
				container.Top(FirstRow...),
				container.Bottom(SecondRow...),
				container.SplitPercent(50),
			),
		}
		ThirdRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParThreeBorder)),
					container.PlaceWidget(newTextThree(ctx, labels[3])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[3]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphThreeBorder)),
					container.PlaceWidget(newPlotThree(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		FourthRow := []container.Option{
			container.SplitVertical(
				container.Left(
					container.Border(draw.LineStyleLight),
					container.BorderTitle("Statistics"),
					container.BorderTitleAlignCenter(),
					container.BorderColor(cell.ColorNumber(ParFourBorder)),
					container.PlaceWidget(newTextFour(ctx, labels[4])),
				),
				container.Right(
					container.Border(draw.LineStyleLight),
					container.BorderTitle(labels[4]+" - Q to quit"),
					container.BorderColor(cell.ColorNumber(GraphFourBorder)),
					container.PlaceWidget(newPlotFour(ctx)),
				),
				container.SplitPercent(15),
			),
		}
		BottomHalf := []container.Option{
			container.SplitHorizontal(
				container.Top(ThirdRow...),
				container.Bottom(FourthRow...),
				container.SplitPercent(50),
			),
		}
		AllRows := []container.Option{
			container.SplitHorizontal(
				container.Top(TopHalf...),
				container.Bottom(BottomHalf...),
				container.SplitPercent(50),
			),
		}
		c, err := container.New(
			t,
			AllRows...,
		)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else {
		err := "Error: Min of 1 column, and Max of 4 columns needed!"
		panic(err)
		return nil, nil
	}
	//if no matches the return nil

}

func initBuffer(records []string) {
	//set buffer to 1440 (minutes in a day)
	plot1buf = ring.NewBuffer(1440)
	plot2buf = ring.NewBuffer(1440)
	plot3buf = ring.NewBuffer(1440)
	plot4buf = ring.NewBuffer(1440)
}

func parsePlotData(records []string) {
	label := records[0]
	record := records[1:]
	if *labelMode == "time" {
		//Use the time as a X-Axis labels
		now := time.Now()
		label = fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())
	} else {
		//Use the records as the X-Axis labels
		label = records[0]
		record = records[1:]
	}
	for i, x := range record {
		if i == 0 {
			if *debug {
				fmt.Println("Record[0]:", record[i])
				fmt.Println("i:", i)
				fmt.Println("x:", x)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot1buf.Add(val, label)
		}
		if i == 1 {
			if *debug {
				fmt.Println("Record[1]:", record[i])
				fmt.Println("i:", i)
				fmt.Println("x:", x)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot2buf.Add(val, label)
		}
		if i == 2 {
			if *debug {
				fmt.Println("Record[2]:", record[i])
				fmt.Println("i:", i)
				fmt.Println("x:", x)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot3buf.Add(val, label)
		}
		if i == 3 {
			if *debug {
				fmt.Println("Record[3]:", record[i])
				fmt.Println("i:", i)
				fmt.Println("x:", x)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot4buf.Add(val, label)
		}
	}
	if *debug {
		fmt.Println("Buffer Data[1]: ", plot1buf.Last(100, 100))
		fmt.Println("Buffer Lables[1]: ", plot1buf.LastLabels(100, 100))
		fmt.Println("Buffer Data[2]: ", plot2buf.Last(100, 100))
		fmt.Println("Buffer Lables[2]: ", plot2buf.LastLabels(100, 100))
		fmt.Println("Buffer Data[3]: ", plot3buf.Last(100, 100))
		fmt.Println("Buffer Lables[3]: ", plot3buf.LastLabels(100, 100))
		fmt.Println("Buffer Data[4]: ", plot4buf.Last(100, 100))
		fmt.Println("Buffer Lables[4]: ", plot4buf.LastLabels(100, 100))

	}
}

// newLinechart returns a line chart that displays a heartbeat-like progression.
func newLinechart(ctx context.Context) *linechart.LineChart {
	var inputs []float64

	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 83% of screen
	termWidth = int((float64(termWidth)*float64(0.83) - 3))
	data1 := plot1buf.Last(termHeight, termWidth)
	//fmt.Println("NEW LINECHARTDATA:", data1)
	for i := 0; i < 100; i++ {
		v := math.Pow(math.Sin(float64(i)), 63) * math.Sin(float64(i)+1.5) * 8
		inputs = append(inputs, v)
	}
	//	strings := []string{"1", "2", "3", "4"}
	//	dataChan <- strings

	//for record := range dataChan {
	//	x, _ := strconv.ParseFloat(record[0], 64)
	//	inputs = append(inputs, x)
	//		fmt.Println(record)
	//}
	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorRed)),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorGreen)),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorGreen)),
	)
	fmt.Println("Float Values", data1)
	step := 0
	go periodic(ctx, redrawInterval/10, func() error {
		step = (step + 1) % len(inputs)
		if err := lc.Series("heartbeat", inputs,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(87))),
			linechart.SeriesXLabels(map[int]string{
				0: "zero",
			}),
		); err != nil {
			return err
		}
		return nil
	})
	return lc
}

//calulate data stats
func prepareStats(buffer *ring.Buffer) string {
	//use only accumulated data instead of ring buffer
	data := buffer.Container
	//data := stats.LoadRawData(records)
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
	count := len(data)
	text := fmt.Sprintf("\nCount:       %d\nMin:         %.2f\nMean:        %.2f\nMedian:      %.2f\nMax:         %.2f\nOutliers:    %.2f",
		count, min, mean, median, max, outlier)

	return text
}

// newTextTime creates a new Text widget that displays the current time.
func newTextOne(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, redrawInterval/5, func() error {
		t.Reset()
		pointer := plot1buf.LastLabels(1, 1)
		value := plot1buf.Last(1, 1)
		data := prepareStats(plot1buf)
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParOneTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
		if *labelMode == "time" {
			now := time.Now()
			pointer[0] = fmt.Sprintf("%02d:%02d:%02d",
				now.Hour(), now.Minute(), now.Second())
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		return nil
	})
	return t
}

func newTextTwo(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, redrawInterval/5, func() error {
		t.Reset()
		pointer := plot2buf.LastLabels(1, 1)
		value := plot2buf.Last(1, 1)
		data := prepareStats(plot2buf)
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParTwoTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
		if *labelMode == "time" {
			now := time.Now()
			pointer[0] = fmt.Sprintf("%02d:%02d:%02d",
				now.Hour(), now.Minute(), now.Second())
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		return nil
	})
	return t
}

func newTextThree(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, redrawInterval/5, func() error {
		t.Reset()
		value := plot3buf.Last(1, 1)
		data := prepareStats(plot3buf)
		pointer := plot3buf.LastLabels(1, 1)
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParThreeTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
		if *labelMode == "time" {
			now := time.Now()
			pointer[0] = fmt.Sprintf("%02d:%02d:%02d",
				now.Hour(), now.Minute(), now.Second())
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		return nil
	})
	return t
}

func newTextFour(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, redrawInterval/5, func() error {
		t.Reset()
		value := plot4buf.Last(1, 1)
		data := prepareStats(plot4buf)
		pointer := plot4buf.LastLabels(1, 1)
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParFourTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
		if *labelMode == "time" {
			now := time.Now()
			pointer[0] = fmt.Sprintf("%02d:%02d:%02d",
				now.Hour(), now.Minute(), now.Second())
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		return nil
	})
	return t
}

// newTextTime creates a new Text widget that displays the current time.
func newTextFourEx(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, redrawInterval/1, func() error {
		//pointer := plot4buf.LastLabels(1, 1)
		t.Reset()
		//data := prepareStats(label, pointer, plot4buf.Buffer())
		//fmt.Println("Paragraph Text", data)
		//txt := time.Now().UTC().Format(time.UnixDate)
		//if err := t.Write(fmt.Sprintf("\n%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorMagenta))); err != nil {
		//	return err
		//}
		return nil
	})
	return t
}

// sineInputs generates values from -1 to 1 for display on the line chart.
func sineInputs() []float64 {
	var res []float64

	for i := 0; i < 200; i++ {
		v := math.Sin(float64(i) / 100 * math.Pi)
		res = append(res, v)
	}
	return res
}

// newSines returns a line chart that displays multiple sine series.
func newPlotOne(ctx context.Context) *linechart.LineChart {
	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 85% of screen
	//termWidth = int((float64(termWidth) * float64(0.30)))
	termWidth = 20

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, redrawInterval/1, func() error {
		var inputs []float64
		var inputLabels []string
		var records []string
		//remove a record from the channel
		records = <-dataChan
		//add record to the buffer
		parsePlotData(records)
		inputs = plot1buf.Last(termHeight, termWidth)
		inputLabels = plot1buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("Channel Value:", records)
			fmt.Println("Data Plot 1 Data:", plot1buf.Last(termHeight, termWidth))
			fmt.Println("Data Plot 1 Data Labels:", plot1buf.LastLabels(termHeight, termWidth))
		}
		//step1 = (step1 + 1) % len(inputs)
		if err := lc.Series("first", inputs,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(GraphLineOne))),
			linechart.SeriesXLabels(labelMap),
		); err != nil {
			return err
		}

		return nil
	})
	return lc
}

// newSines returns a line chart that displays multiple sine series.
func newPlotTwo(ctx context.Context) *linechart.LineChart {
	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 85% of screen
	termWidth = int((float64(termWidth) * float64(0.90)))
	termWidth = 20

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, redrawInterval/1, func() error {
		var inputs []float64
		var inputLabels []string
		inputs = plot2buf.Last(termHeight, termWidth)
		inputLabels = plot2buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("Data Plot 2 Data:", plot2buf.Last(termHeight, termWidth))
			fmt.Println("Data Plot 2 Data Labels:", plot2buf.LastLabels(termHeight, termWidth))
		}
		//step1 = (step1 + 1) % len(inputs)
		if err := lc.Series("first", inputs,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(GraphLineTwo))),
			linechart.SeriesXLabels(labelMap),
		); err != nil {
			return err
		}

		return nil
	})
	return lc
}

// newSines returns a line chart that displays multiple sine series.
func newPlotThree(ctx context.Context) *linechart.LineChart {
	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 85% of screen
	termWidth = int((float64(termWidth) * float64(0.80)))
	termWidth = 20

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, redrawInterval/1, func() error {
		var inputs []float64
		var inputLabels []string
		inputs = plot3buf.Last(termHeight, termWidth)
		inputLabels = plot3buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("Data Plot 3 Data:", plot3buf.Last(termHeight, termWidth))
			fmt.Println("Data Plot 3 Data Labels:", plot3buf.LastLabels(termHeight, termWidth))
		}
		//step1 = (step1 + 1) % len(inputs)
		if err := lc.Series("first", inputs,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(GraphLineThree))),
			linechart.SeriesXLabels(labelMap),
		); err != nil {
			return err
		}

		return nil
	})
	return lc
}

// newSines returns a line chart that displays multiple sine series.
func newPlotFour(ctx context.Context) *linechart.LineChart {
	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 85% of screen
	termWidth = int((float64(termWidth) * float64(0.70)))
	termWidth = 20

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, redrawInterval/1, func() error {
		var inputs []float64
		var inputLabels []string
		inputs = plot4buf.Last(termHeight, termWidth)
		inputLabels = plot4buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("Data Plot 4 Data:", plot4buf.Last(termHeight, termWidth))
			fmt.Println("Data Plot 4 Data Labels:", plot4buf.LastLabels(termHeight, termWidth))
		}
		//step1 = (step1 + 1) % len(inputs)
		if err := lc.Series("first", inputs,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(GraphLineFour))),
			linechart.SeriesXLabels(labelMap),
		); err != nil {
			return err
		}

		return nil
	})
	return lc
}

// periodic executes the provided closure periodically every interval.
// Exits when the context expires.
func periodic(ctx context.Context, interval time.Duration, fn func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := fn(); err != nil {
				panic(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// rotate returns a new slice with inputs rotated by step.
// I.e. for a step of one:
//   inputs[0] -> inputs[len(inputs)-1]
//   inputs[1] -> inputs[0]
// And so on.
func rotate(inputs []float64, step int) []float64 {
	return append(inputs[step:], inputs[:step]...)
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
	go func() {
		for {
			if faster == true {
				//time.Sleep(30 * time.Millisecond)
			}
			if reset == true {
				//time.Sleep(70 * time.Millisecond)
			}
			if slower == true {
				//time.Sleep(350 * time.Millisecond)
			}
			if interrupt == true {
				//time.Sleep(10 * time.Second)
				interrupt = false
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

	////////////// END INIT TERMDASH  ///////////////////
	//theme colors
	//termuiColors()
	//init the widgets
	initBuffer(records)
	//setup the grid
	//setupGrid()

	if *debug == false {
		t, err := termbox.New(termbox.ColorMode(terminalapi.ColorMode256))
		if err != nil {
			panic(err)
		}
		defer t.Close()

		ctx, cancel := context.WithCancel(context.Background())
		c, err := layout(ctx, t, labels)
		if err != nil {
			panic(err)
		}

		quitter := func(k *terminalapi.Keyboard) {
			if k.Key == 'q' || k.Key == 'Q' {
				cancel()
			}
			if k.Key == keyboard.KeyArrowLeft || k.Key == 'f' {
				faster = true
			}
			if k.Key == keyboard.KeyArrowRight || k.Key == 's' {
				slower = true
			}
			if k.Key == keyboard.KeySpace || k.Key == 'p' {
				interrupt = true
			}
		}
		if err := termdash.Run(ctx, t, c, termdash.KeyboardSubscriber(quitter), termdash.RedrawInterval(redrawInterval)); err != nil {
			panic(err)
		}
	}

}
