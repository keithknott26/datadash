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
	app            = kingpin.New("datadash", "A CCS Graphing Application")
	debug          = app.Flag("debug", "Enable Debug Mode").Bool()
	delimiter      = app.Flag("delimiter", "Record Delimiter:").Short('d').Default("\t").String()
	labelMode      = app.Flag("label-mode", "X-Axis Labels: 'first' (use the first record in the column) or 'time' (use the current time)").Short('m').Default("first").String()
	hScale         = app.Flag("horizontal-scale", "Horizontal Graph Scale (Line graph width * X): (1,2,3,...)").Short('h').Default("1").Int()
	redrawInterval = app.Flag("redraw-interval", "The interval at which objects on the screen are redrawn: (100ms,250ms,1s,5s..)").Short('r').Default("100ms").Duration()
	seekInterval   = app.Flag("seek-interval", "The interval at which records (lines) are read from the datasource: (100ms,250ms,1s,5s..)").Short('l').Default("100ms").Duration()
	inputFile      = app.Arg("input file", "A file containing a label header, and data in columns separated by delimiter 'd'.\nData piped from Stdin uses the same format").File()

	plot0buf *ring.Buffer
	plot1buf *ring.Buffer
	plot2buf *ring.Buffer
	plot3buf *ring.Buffer
	plot4buf *ring.Buffer

	ctx context.Context
	//to be removed
	//keep
	dataChan      = make(chan []string, 5)
	labels        = make([]string, 0, 10)
	graphs        = 1
	termWidth     = 1
	termHeight    = 1
	linchartWidth = 1
	drawOffset    = 1
	//speed controls
	slower    = false
	faster    = false
	interrupt = false
	resume    = false
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

	ParOneBorder     = 26
	ParOneTitle      = 82
	ParTwoBorder     = 25
	ParTwoTitle      = 13
	ParThreeBorder   = 25
	ParThreeTitle    = 45
	ParFourBorder    = 25
	ParFourTitle     = 9
	ParText          = 3
	ParPointer       = 248
	ParValue         = 15
	LineLow          = 2
	LineHigh         = 1
	GraphOneBorder   = 25
	GraphTwoBorder   = 25
	GraphThreeBorder = 25
	GraphFourBorder  = 25
	GraphAxes        = 8
	GraphXLabels     = 248
	GraphYLabels     = 15
	GraphTitles      = 2
	GraphLineOne     = 82
	GraphLineTwo     = 13
	GraphLineThree   = 45
	GraphLineFour    = 9
	GraphLinePaused  = 1
)

func layout(ctx context.Context, t terminalapi.Terminal, labels []string) (*container.Container, error) {
	var labels0 string
	var labels1 string
	var labels2 string
	var labels3 string
	var labels4 string
	if graphs == 0 && *labelMode == "time" {
		labels0 = "Streaming Data..."
		labels1 = "Empty"
		labels2 = "Empty"
		labels3 = "Empty"
		labels4 = "Empty"
	} else if graphs == 1 {
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = "Empty"
		labels3 = "Empty"
		labels4 = "Empty"
	} else if graphs == 2 {
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = labels[2]
		labels3 = "Empty"
		labels4 = "Empty"
	} else if graphs == 3 {
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = labels[2]
		labels3 = labels[3]
		labels4 = "Empty"
	} else if graphs == 4 {
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = labels[2]
		labels3 = labels[3]
		labels4 = labels[4]
	}
	StreamingDataRow := []container.Option{
		container.SplitVertical(
			container.Left(
				container.Border(draw.LineStyleLight),
				container.BorderTitle("Statistics"),
				container.BorderTitleAlignCenter(),
				container.BorderColor(cell.ColorNumber(ParOneBorder)),
				container.PlaceWidget(newTextOne(ctx, labels0)),
			),
			container.Right(
				container.Border(draw.LineStyleLight),
				container.BorderTitle(labels0+" - 'q' Quit 'p' Pause 10s <- Slow | Resume -> "),
				container.BorderColor(cell.ColorNumber(GraphOneBorder)),
				container.PlaceWidget(newPlotStreamingData(ctx)),
			),
			container.SplitPercent(15),
		),
	}
	FirstRow := []container.Option{
		container.SplitVertical(
			container.Left(
				container.Border(draw.LineStyleLight),
				container.BorderTitle("Statistics"),
				container.BorderTitleAlignCenter(),
				container.BorderColor(cell.ColorNumber(ParOneBorder)),
				container.PlaceWidget(newTextOne(ctx, labels1)),
			),
			container.Right(
				container.Border(draw.LineStyleLight),
				container.BorderTitle(labels1+" - 'q' Quit 'p' Pause 10s <- Slow | Resume -> "),
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
				container.PlaceWidget(newTextTwo(ctx, labels2)),
			),
			container.Right(
				container.Border(draw.LineStyleLight),
				container.BorderTitle(labels2+" - 'q' Quit 'p' Pause 10s <- Slow | Resume -> "),
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
				container.PlaceWidget(newTextThree(ctx, labels3)),
			),
			container.Right(
				container.Border(draw.LineStyleLight),
				container.BorderTitle(labels3+" - 'q' Quit 'p' Pause 10s <- Slow | Resume -> "),
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
				container.PlaceWidget(newTextFour(ctx, labels4)),
			),
			container.Right(
				container.Border(draw.LineStyleLight),
				container.BorderTitle(labels4+" - 'q' Quit 'p' Pause 10s <- Slow | Resume -> "),
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
	if graphs == 0 && *labelMode == "time" {
		c, err := container.New(t, StreamingDataRow...)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if graphs == 1 {
		c, err := container.New(t, FirstRow...)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if graphs == 2 {
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

	} else if graphs == 3 {
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
	} else if graphs == 4 {
		c, err := container.New(
			t,
			AllRows...,
		)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else {
		err := "\n\nError: Columns Detected: " + strconv.Itoa(graphs)
		text := err + "\n\nError: This app wants a minimum of 2 columns and a maximum of 5 columns. You must include a header record:\n\n\t\tHeader record:\tIgnored<delimiter>Title\n\t\tData Row:\tX-Label<delimiter>Y-value\n\n\n\nExample:  \n\t\ttime\tADL Inserts\n\t\t00:01\t493\n\t\t00:02\t353\n\t\t00:03\t380\n\nExample:\n\t\tcol1\tcol2\n\t\t1\t493\n\t\t2\t353\n\t\t3\t321\n"

		panic(text)
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
	var label string
	var record []string
	if graphs == 0 {
		record = records[0:]
	} else {
		label = records[0]
		record = records[1:]
	}
	if *labelMode == "time" {
		//Use the time as a X-Axis labels
		now := time.Now()
		label = fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())
	}

	for i, x := range record {
		if *debug {
			fmt.Println("DEBUG:\tFull Record:", record)
		}
		if i == 0 {
			if *debug {
				fmt.Println("DEBUG:\tRecord[0]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot1buf.Add(val, label)
		}
		if i == 1 {
			if *debug {
				fmt.Println("DEBUG:\tRecord[1]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot2buf.Add(val, label)
		}
		if i == 2 {
			if *debug {
				fmt.Println("DEBUG:\tRecord[2]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot3buf.Add(val, label)
		}
		if i == 3 {
			if *debug {
				fmt.Println("DEBUG:\tRecord[3]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			plot4buf.Add(val, label)
		}
	}
	if *debug {
		fmt.Println("DEBUG:\tBuffer Data[1]: ", plot1buf.Last(100, 100))
		fmt.Println("DEBUG:\tBuffer Lables[1]: ", plot1buf.LastLabels(100, 100))
		fmt.Println("DEBUG:\tBuffer Data[2]: ", plot2buf.Last(100, 100))
		fmt.Println("DEBUG:\tBuffer Lables[2]: ", plot2buf.LastLabels(100, 100))
		fmt.Println("DEBUG:\tBuffer Data[3]: ", plot3buf.Last(100, 100))
		fmt.Println("DEBUG:\tBuffer Lables[3]: ", plot3buf.LastLabels(100, 100))
		fmt.Println("DEBUG:\tBuffer Data[4]: ", plot4buf.Last(100, 100))
		fmt.Println("DEBUG:\tBuffer Lables[4]: ", plot4buf.LastLabels(100, 100))

	}
}

// newLinechart returns a line chart that displays a heartbeat-like progression.
func newLinechart(ctx context.Context) *linechart.LineChart {
	var inputs []float64

	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 83% of screen
	termWidth = int((float64(termWidth)*float64(0.83) - 3))
	data1 := plot1buf.Last(termHeight, termWidth)
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
	fmt.Println("DEBUG:\tFloat Values", data1)
	step := 0
	go periodic(ctx, *redrawInterval/10, func() error {
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
	if *debug {
		fmt.Println("DEBUG:\tprepareStats Function: Execute")
	}
	//use only accumulated data instead of ring buffer
	data := buffer.Container
	outliers, _ := stats.QuartileOutliers(data)
	median, _ := stats.Median(data)
	mean, _ := stats.Mean(data)
	min, _ := stats.Min(data)
	max, _ := stats.Max(data)
	//variance, _ := stats.Variance(data)
	//stddev, _ := stats.StandardDeviation(data)
	var outlierStr string
	var outliersArr []float64

	//create the outliers slice
	for _, v := range outliers.Extreme {
		outliersArr = append(outliersArr, v)
	}
	//overwrite the outliers slice with the outliers
	for _, v := range outliersArr {
		outliersArr = append(outliersArr, v)
	}

	//reverse order so largest outliers appear first
	for i := len(outliersArr)/2 - 1; i >= 0; i-- {
		opp := len(outliersArr) - 1 - i
		outliersArr[i], outliersArr[opp] = outliersArr[opp], outliersArr[i]
	}
	for i, v := range outliersArr {
		if i == 0 || i == 1 || i == 2 {
			s := fmt.Sprintf("%.2f", v)
			outlierStr = outlierStr + s + "\n             "
		}
	}
	//count records
	count := len(data)
	text := fmt.Sprintf("\nCount:       %d\nMin:         %.2f\nMean:        %.2f\nMedian:      %.2f\nMax:         %.2f\nOutliers:    %s",
		count, min, mean, median, max, outlierStr)

	return text
}

// newTextTime creates a new Text widget that displays the current time.
func newTextOne(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, *redrawInterval*2, func() error {
		pointer := plot1buf.LastLabels(1, 1)
		value := plot1buf.Last(1, 1)
		data := prepareStats(plot1buf)
		t.Reset()
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParOneTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
		return nil
	})
	return t
}

func newTextTwo(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, *redrawInterval*2, func() error {
		pointer := plot2buf.LastLabels(1, 1)
		value := plot2buf.Last(1, 1)
		data := prepareStats(plot2buf)
		t.Reset()
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParTwoTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
		return nil
	})
	return t
}

func newTextThree(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, *redrawInterval*2, func() error {
		value := plot3buf.Last(1, 1)
		data := prepareStats(plot3buf)
		pointer := plot3buf.LastLabels(1, 1)
		t.Reset()
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParThreeTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
		return nil
	})
	return t
}

func newTextFour(ctx context.Context, label string) *text.Text {
	t := text.New()
	go periodic(ctx, *redrawInterval*2, func() error {

		value := plot4buf.Last(1, 1)
		data := prepareStats(plot4buf)
		pointer := plot4buf.LastLabels(1, 1)
		t.Reset()
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParFourTitle)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParValue)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParText)))); err != nil {
			return err
		}
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

func readDataChannel(ctx context.Context) {
	go periodic(ctx, *seekInterval, func() error {
		var records []string
		//remove a record from the channel
		if *debug {
			fmt.Println("DEBUG:\tRemoving record from channel.")
		}
		records = <-dataChan
		//add record to the buffer
		if *debug {
			fmt.Println("DEBUG:\tParsing line record:", records)
		}
		parsePlotData(records)
		return nil
	})
}

// returns a line chart that displays data from column 2
func newPlotStreamingData(ctx context.Context) *linechart.LineChart {
	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 85% of screen
	termWidth = int((float64(termWidth) * float64(0.85))) * *hScale

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, time.Duration(10*time.Millisecond), func() error {
		var inputs []float64
		var inputLabels []string

		inputs = plot1buf.Last(termHeight, termWidth)
		inputLabels = plot1buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("DEBUG:\tData Plot 1 Data (Streaming Data):", plot1buf.Last(termHeight, termWidth))
			fmt.Println("DEBUG:\tData Plot 1 Data Labels (Streaming Data):", plot1buf.LastLabels(termHeight, termWidth))
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

// returns a line chart that displays data from column 2
func newPlotOne(ctx context.Context) *linechart.LineChart {
	termWidth, termHeight := ui.TerminalDimensions()
	//set termWidth to actual width of linechart 85% of screen
	termWidth = int((float64(termWidth) * float64(0.85))) * *hScale

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, time.Duration(10*time.Millisecond), func() error {
		var inputs []float64
		var inputLabels []string

		inputs = plot1buf.Last(termHeight, termWidth)
		inputLabels = plot1buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("DEBUG:\tData Plot 1 Data:", plot1buf.Last(termHeight, termWidth))
			fmt.Println("DEBUG:\tData Plot 1 Data Labels:", plot1buf.LastLabels(termHeight, termWidth))
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
	termWidth = int((float64(termWidth) * float64(0.85))) * *hScale
	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, *redrawInterval/1, func() error {
		var inputs []float64
		var inputLabels []string
		inputs = plot2buf.Last(termHeight, termWidth)
		inputLabels = plot2buf.LastLabels(termHeight, termWidth)
		//add every other label
		//y := inputLabels[:0]
		//for i, n := range inputLabels {
		//	if i%5 != 0 {
		//		y = append(y, n)
		//	}
		//}

		labelMap := make(map[int]string)
		for i, s := range inputLabels {
			labelMap[i] = s
		}

		if *debug {
			fmt.Println("DEBUG:\tData Plot 2 Label Map:", labelMap)
			fmt.Println("DEBUG:\tData Plot 2 Data:", plot2buf.Last(termHeight, termWidth))
			fmt.Println("DEBUG:\tData Plot 2 Data Labels:", plot2buf.LastLabels(termHeight, termWidth))
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
	termWidth = int((float64(termWidth) * float64(0.85))) * *hScale

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, *redrawInterval/1, func() error {
		var inputs []float64
		var inputLabels []string
		inputs = plot3buf.Last(termHeight, termWidth)
		inputLabels = plot3buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("DEBUG:\tData Plot 3 Data:", plot3buf.Last(termHeight, termWidth))
			fmt.Println("DEBUG:\tData Plot 3 Data Labels:", plot3buf.LastLabels(termHeight, termWidth))
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
	termWidth = int((float64(termWidth) * float64(0.85))) * *hScale

	lc := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
	)
	//step1 := 0
	go periodic(ctx, *redrawInterval/1, func() error {
		var inputs []float64
		var inputLabels []string
		inputs = plot4buf.Last(termHeight, termWidth)
		inputLabels = plot4buf.LastLabels(termHeight, termWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}
		if *debug {
			fmt.Println("DEBUG:\tData Plot 4 Data:", plot4buf.Last(termHeight, termWidth))
			fmt.Println("DEBUG:\tData Plot 4 Data Labels:", plot4buf.LastLabels(termHeight, termWidth))
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
	interrupt := false

	// Parse args and assign values
	kingpin.Version("0.0.1")
	kingpin.MustParse(app.Parse(os.Args[1:]))
	if *debug {
		fmt.Printf("DEBUG:\tRunning with: Delimiter: '%s'\nlabelMode: %s\nReDraw Interval: %s\nSeek Interval: %s\n", *delimiter, *labelMode, *redrawInterval, *seekInterval)
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
		fmt.Println("DEBUG:\tRecords Array:", records)
		fmt.Println("DEBUG:\tNumber of Graphs:", graphs)
		fmt.Println("DEBUG:\tLabels Array:", labels)
	}
	// read from Reader (Stdin or File) into a dataChan
	go func() {
		for {
			if reset == true {
				//time.Sleep(*seekInterval * 1)
			}
			if faster == true {
				//time.Sleep(*seekInterval * 1)

				//	seekInterval = time.Duration(50*time.Millisecond) * time.Duration(*readInterval)
				//	redrawInterval = time.Duration(50*time.Millisecond) * time.Duration(*drawInterval)
			}
			if slower == true {
				time.Sleep(*seekInterval * 5)
				//time.Sleep(500 * time.Millisecond)
			}
			if interrupt == true {
				time.Sleep(10 * time.Second)
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
	}() //end read from stdin/file

	//initialize the ring buffer
	initBuffer(records)
	//Initialize termbox in 256 color mode
	t, err := termbox.New(termbox.ColorMode(terminalapi.ColorMode256))
	if err != nil {
		panic(err)
	}
	defer t.Close()

	//configure the box / graph layout
	ctx, cancel := context.WithCancel(context.Background())
	c, err := layout(ctx, t, labels)
	if err != nil {
		panic(err)
	}
	//start reading from the data channel
	readDataChannel(ctx)
	//listen for keyboard events
	keyboardevents := func(k *terminalapi.Keyboard) {
		if k.Key == 'q' || k.Key == 'Q' {
			cancel()
		}
		if k.Key == keyboard.KeyArrowLeft || k.Key == 'f' {
			slower = true
			faster = false
			reset = false
		}
		if k.Key == keyboard.KeyArrowRight || k.Key == 's' {
			faster = true
			slower = false
			reset = false
		}
		if k.Key == 'p' {
			interrupt = true
			slower = false
			faster = false
		}
		if k.Key == keyboard.KeySpace {
			reset = true
			slower = false
			faster = false
		}
	}
	if err := termdash.Run(ctx, t, c, termdash.KeyboardSubscriber(keyboardevents), termdash.RedrawInterval(*redrawInterval)); err != nil {
		panic(err)
	}
} //end main
