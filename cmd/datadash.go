package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	termutil "github.com/andrew-d/go-termutil"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/terminal/termbox"
	"github.com/mum4k/termdash/terminal/terminalapi"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/keithknott26/datadash"
)

const (
	BUFFER_SIZE = 1440
)

var (
	app            = kingpin.New("datadash", "A Data Visualization Tool")
	debug          = app.Flag("debug", "Enable Debug Mode").Bool()
	delimiter      = app.Flag("delimiter", "Record Delimiter. Default: \t").Short('d').Default("\t").String()
	labelMode      = app.Flag("label-mode", "X-Axis Labels: 'first' (use the first record in the column) or 'time' (use the current time)").Short('m').Default("first").String()
	scrollData     = app.Flag("scroll", "Whether or not to scroll chart data (true, false). Default: false").Short('s').Default("false").Bool()
	avgLine        = app.Flag("average-line", "Enables the line representing the average of values. Default: false").Short('a').Default("false").Bool()
	avgSeek        = app.Flag("average-seek", "The number of values to consider when displaying the average line: (50,100,500...) Default: 500").Short('z').Default("500").Int()
	yAxisAdaptive  = app.Flag("adaptive-y", "Makes the Y axis adapt its base value depending on the provided series. Without this option, the Y axis always starts at the zero value regardless of values available in the series. Default: false").Short('y').Default("false").Bool()
	graphType      = app.Flag("graph-type", "The type of graphs to display (line, bar, spark). Default: line").Short('g').Default("line").String()
	redrawInterval = app.Flag("redraw-interval", "The interval at which objects on the screen are redrawn: (100ms,250ms,1s,5s..) Default 10ms").Short('r').Default("10ms").Duration()
	seekInterval   = app.Flag("seek-interval", "The interval at which records (lines) are read from the datasource: (100ms,250ms,1s,5s..) Default: 20ms").Short('l').Default("20ms").Duration()
	inputFile      = app.Arg("input file", "A file containing a label header, and data in columns separated by delimiter 'd'.\nData piped from Stdin uses the same format").File()

	ctx    context.Context
	stream *datadash.Row

	row1 *datadash.Row
	row2 *datadash.Row
	row3 *datadash.Row
	row4 *datadash.Row
	row5 *datadash.Row

	dataChan = make(chan []string, 10)
	labels   = make([]string, 0, 0)
	graphs   = 1
	//scrolling speed control
	slower    = false
	faster    = false
	interrupt = false
	resume    = false
)

func layout(ctx context.Context, t terminalapi.Terminal, labels []string) (*container.Container, error) {
	var labels0 string
	var labels1 string
	var labels2 string
	var labels3 string
	var labels4 string
	var labels5 string
	switch graphs {
	case 0:
		labels0 = "Streaming Data..."
		labels1 = "Empty"
		labels2 = "Empty"
		labels3 = "Empty"
		labels4 = "Empty"
		labels5 = "Empty"
		*labelMode = "time"
	case 1:
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = "Empty"
		labels3 = "Empty"
		labels4 = "Empty"
		labels5 = "Empty"
	case 2:
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = labels[2]
		labels3 = "Empty"
		labels4 = "Empty"
		labels5 = "Empty"
	case 3:
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = labels[2]
		labels3 = labels[3]
		labels4 = "Empty"
		labels5 = "Empty"
	case 4:
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = labels[2]
		labels3 = labels[3]
		labels4 = labels[4]
		labels5 = "Empty"
	case 5:
		labels0 = labels[0]
		labels1 = labels[1]
		labels2 = labels[2]
		labels3 = labels[3]
		labels4 = labels[4]
		labels5 = labels[5]
	}

	//types := make([]rune, len(labels), len(labels))
	//copy(types, []rune(strings.ToUpper(*panelTypes)))
	//for i, label := range labels {
	//	var panelType int
	//	switch types[i] {
	//	case 'L':
	//		panelType = termeter.LINE
	//	case 'D':
	//		panelType = termeter.CDF
	//	case 'C':
	//		panelType = termeter.COUNTER
	//	default:
	///		graphType = "L"
	//		//Initialize Row
	//		stream.InitWidgets(ctx, labels0, *redrawInterval)
	//		stream.Context = ctx
	//		StreamingDataRow := stream.ContainerOptions(stream.Context, graphType)
	//		if err != nil {
	//			panelType = termeter.COUNTER
	//		}
	//	}
	//	app.AddPanel(label, panelType, options)
	//}

	//Initialize Rows (up to 5 are supported)
	stream.InitWidgets(ctx, *graphType, labels0, *redrawInterval, *seekInterval)
	stream.Context = ctx
	StreamingDataRow := stream.ContainerOptions(stream.Context, *graphType)

	row1.InitWidgets(ctx, *graphType, labels1, *redrawInterval, *seekInterval)
	row1.Context = ctx
	FirstRow := row1.ContainerOptions(row1.Context, *graphType)

	row2.InitWidgets(ctx, *graphType, labels2, *redrawInterval, *seekInterval)
	row2.Context = ctx
	SecondRow := row2.ContainerOptions(row2.Context, *graphType)

	row3.InitWidgets(ctx, *graphType, labels3, *redrawInterval, *seekInterval)
	row3.Context = ctx
	ThirdRow := row3.ContainerOptions(row3.Context, *graphType)

	row4.InitWidgets(ctx, *graphType, labels4, *redrawInterval, *seekInterval)
	row4.Context = ctx
	FourthRow := row4.ContainerOptions(row4.Context, *graphType)

	row5.InitWidgets(ctx, *graphType, labels5, *redrawInterval, *seekInterval)
	row5.Context = ctx
	FifthRow := row5.ContainerOptions(row5.Context, *graphType)

	TopHalf := []container.Option{
		container.SplitHorizontal(
			container.Top(FirstRow...),
			container.Bottom(SecondRow...),
			container.SplitPercent(50),
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

	switch graphs {
	case 0:
		c, err := container.New(t, StreamingDataRow...)
		if err != nil {
			return nil, err
		}
		return c, nil
	case 1:
		c, err := container.New(t, FirstRow...)
		if err != nil {
			return nil, err
		}
		return c, nil
	case 2:
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
	case 3:
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
	case 4:
		c, err := container.New(
			t,
			AllRows...,
		)
		if err != nil {
			return nil, err
		}
		return c, nil
	case 5:
		c, err := container.New(
			t,
			container.SplitHorizontal(
				container.Top(AllRows...),
				container.Bottom(FifthRow...),
				container.SplitPercent(80),
			),
		)
		if err != nil {
			return nil, err
		}
		return c, nil
	default:
		err := "\n\nError: Columns Detected: " + strconv.Itoa(graphs)
		text := err + "\n\nError: This app wants a minimum of 2 columns and a maximum of 5 columns. You must include a header record:\n\n\t\tHeader record:\tIgnored<delimiter>Title\n\t\tData Row:\tX-Label<delimiter>Y-value\n\n\n\nExample:  \n\t\ttime\tADL Inserts\n\t\t00:01\t493\n\t\t00:02\t353\n\t\t00:03\t380\n\nExample:\n\t\tcol1\tcol2\n\t\t1\t493\n\t\t2\t353\n\t\t3\t321\n"

		panic(text)
	}

	//if no matches the return nil

}

func initBuffer(records []string) {
	//initialize the rows
	stream = datadash.NewRow(ctx, "Streaming Data...", BUFFER_SIZE, 0, *scrollData, *avgLine, *yAxisAdaptive)
	row1 = datadash.NewRow(ctx, "Row1...", BUFFER_SIZE, 1, *scrollData, *avgLine, *yAxisAdaptive)
	row2 = datadash.NewRow(ctx, "Row2...", BUFFER_SIZE, 2, *scrollData, *avgLine, *yAxisAdaptive)
	row3 = datadash.NewRow(ctx, "Row3...", BUFFER_SIZE, 3, *scrollData, *avgLine, *yAxisAdaptive)
	row4 = datadash.NewRow(ctx, "Row4...", BUFFER_SIZE, 4, *scrollData, *avgLine, *yAxisAdaptive)
	row5 = datadash.NewRow(ctx, "Row5...", BUFFER_SIZE, 5, *scrollData, *avgLine, *yAxisAdaptive)
}

func parsePlotData(records []string) {
	var label string
	var record []string

	//streaming data mode or normal mode
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
		switch i {
		case 0:
			if *debug {
				fmt.Println("DEBUG:\tRecord[0]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			stream.Update(val, label, *avgSeek)
			row1.Update(val, label, *avgSeek)

		case 1:
			if *debug {
				fmt.Println("DEBUG:\tRecord[1]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			row2.Update(val, label, *avgSeek)

		case 2:
			if *debug {
				fmt.Println("DEBUG:\tRecord[2]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			row3.Update(val, label, *avgSeek)

		case 3:
			if *debug {
				fmt.Println("DEBUG:\tRecord[3]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			row4.Update(val, label, *avgSeek)

		case 4:
			if *debug {
				fmt.Println("DEBUG:\tRecord[5]:", record[i])
				fmt.Println("DEBUG:\tCount Value[i]:", i)
				fmt.Println("DEBUG:\tRecord Value [x]:", x)
				fmt.Println("DEBUG:\tLabel Value:", label)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			row5.Update(val, label, *avgSeek)
		}
	}

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
//
//	inputs[0] -> inputs[len(inputs)-1]
//	inputs[1] -> inputs[0]
//
// And so on.
func rotate(inputs []float64, step int) []float64 {
	return append(inputs[step:], inputs[:step]...)
}

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
		fmt.Printf("DEBUG:\tRunning with: Delimiter: '%s'\nlabelMode: %s\nReDraw Interval: %s\nSeek Interval: %s\n, Scrolling: %t\nDisplay Average Line: %t\n yAxisAdaptive: %t\n", *delimiter, *labelMode, *redrawInterval, *seekInterval, *scrollData, *avgLine, *yAxisAdaptive)
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
				time.Sleep(*seekInterval * 4)
			}
			if faster == true {
				time.Sleep(*seekInterval * 1)

			}
			if slower == true {
				time.Sleep(*seekInterval * 6)
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

	//initialize the ring buffer and widgets
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
