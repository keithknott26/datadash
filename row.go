package datadash

import (
	"context"
	"fmt"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/text"
)

type Panel interface {
	NewRow() *Row
	InitWidgets() *Row
	NewContainer() []container.Option
	Update()
}

// row types
const (
	SCROLLING = iota
	LINE
	BAR
)

// sort mode of counter panel
const (
	SORT_NONE = iota
	SORT_ALPHABETICAL
	SORT_NUMERICAL
)

var (
	ParOneBorder     = 26
	ParOneTitle      = 82
	ParTwoBorder     = 25
	ParTwoTitle      = 13
	ParThreeBorder   = 25
	ParThreeTitle    = 45
	ParFourBorder    = 25
	ParFourTitle     = 9
	ParFiveTitle     = 165
	ParFiveBorder    = 25
	ParText          = 3
	ParPointer       = 248
	ParValue         = 15
	GraphOneBorder   = 25
	GraphTwoBorder   = 25
	GraphThreeBorder = 25
	GraphFourBorder  = 25
	GraphFiveBorder  = 25
	GraphAxes        = 8
	GraphXLabels     = 248
	GraphYLabels     = 15
	GraphTitles      = 2
	LineLow          = 235
	LineAvg          = 235
	LineHigh         = 239
	GraphLineOne     = 82
	GraphLineTwo     = 13
	GraphLineThree   = 45
	GraphLineFour    = 9
	GraphLineFive    = 165
	GraphLinePaused  = 1
	ctx              context.Context
)

//type RowInterface interface {
//	NewRow()      // same as adding the methods of ReadWriter
//	InitWidgets() // same as adding the methods of Locker
//	returnChart()
//}

type Row struct {
	Id               int
	Label            string
	Scroll           bool
	Average          bool
	Labels           *stringRingBuffer
	Context          context.Context
	Data             *float64RingBuffer
	Averages         *float64RingBuffer
	Chart            *linechart.LineChart
	Textbox          *text.Text
	DataContainer    []float64
	LabelContainer   []string
	AverageContainer []float64
}

//func (self *Row) increment() {
//	self.id++
//}

func NewRow(ctx context.Context, label string, bufsize int, id int, scroll bool, average bool) *Row {
	row := &Row{
		Id:       id,
		Scroll:   scroll,
		Average:  average,
		Label:    label,
		Context:  ctx,
		Data:     newFloat64RingBuffer(bufsize),
		Labels:   newStringRingBuffer(bufsize),
		Averages: newFloat64RingBuffer(bufsize),
	}
	return row
}

func (r *Row) InitWidgets(ctx context.Context, label string) *Row {
	r.Label = label
	r.Textbox = r.newTextBox(ctx, label)
	r.Chart = r.newLineChart(ctx)
	return r
}

func (r *Row) newLineChart(ctx context.Context) *linechart.LineChart {
	lc, err := r.newStreamingLineChart(ctx)
	if err != nil {
		fmt.Println("LineChart Error:", err)
	}
	return lc
}
func (r *Row) newTextBox(ctx context.Context, label string) *text.Text {
	t, err := r.newText(ctx, label)
	if err != nil {
		fmt.Println("TextBox Error:", err)
	}
	return t
}

func (r *Row) NewContainer(ctx context.Context, label string) []container.Option {
	containerOptions := r.ContainerOptions(ctx)
	return containerOptions
}
func (r *Row) ContainerOptions(ctx context.Context) []container.Option {
	var ParBorder int
	var GraphBorder int
	if r.Id == 0 {
		GraphBorder = GraphOneBorder
		ParBorder = ParOneBorder
	} else if r.Id == 1 {
		GraphBorder = GraphOneBorder
		ParBorder = ParOneBorder
	} else if r.Id == 2 {
		GraphBorder = GraphTwoBorder
		ParBorder = ParTwoBorder
	} else if r.Id == 3 {
		GraphBorder = GraphThreeBorder
		ParBorder = ParThreeBorder
	} else if r.Id == 4 {
		GraphBorder = GraphFourBorder
		ParBorder = ParFourBorder
	} else if r.Id == 5 {
		GraphBorder = GraphFiveBorder
		ParBorder = ParFiveBorder
	}
	row := []container.Option{
		container.SplitVertical(
			container.Left(
				container.Border(linestyle.Round),
				container.BorderTitle("Statistics"),
				container.BorderTitleAlignCenter(),
				container.BorderColor(cell.ColorNumber(ParBorder)),
				container.PlaceWidget(r.Textbox),
			),
			container.Right(
				container.Border(linestyle.Round),
				container.BorderTitle(r.Label+" - 'q' Quit 'p' Pause 10s <- Slow | Resume -> | Scroll to Zoom..."),
				container.BorderColor(cell.ColorNumber(GraphBorder)),
				container.PlaceWidget(r.Chart),
			),
			container.SplitPercent(15),
		)}
	return row
}

func (r *Row) newText(ctx context.Context, label string) (*text.Text, error) {
	var ParTitle int
	if r.Id == 0 {
		ParTitle = ParOneTitle
	} else if r.Id == 1 {
		ParTitle = ParOneTitle
	} else if r.Id == 2 {
		ParTitle = ParTwoTitle
	} else if r.Id == 3 {
		ParTitle = ParThreeTitle
	} else if r.Id == 4 {
		ParTitle = ParFourTitle
	} else if r.Id == 5 {
		ParTitle = ParFiveTitle
	}

	t, err := text.New()
	context := ctx
	go periodic(context, time.Duration(100*time.Millisecond)/2, func() error {
		defer func() {
			recover()
		}()
		pointer := r.Labels.Last(1)
		value := r.Data.Last(1)
		data := prepareStats(r, r.Data)
		t.Reset()
		if err := t.Write(fmt.Sprintf("%s", label), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(ParTitle)))); err != nil {
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
	return t, err
}

func (r *Row) newStreamingLineChart(ctx context.Context) (*linechart.LineChart, error) {
	//set the line color based on the r.Id
	var GraphLine int
	var lc *linechart.LineChart
	var err error
	if r.Id == 0 {
		GraphLine = GraphLineOne
	} else if r.Id == 1 {
		GraphLine = GraphLineOne
	} else if r.Id == 2 {
		GraphLine = GraphLineTwo
	} else if r.Id == 3 {
		GraphLine = GraphLineThree
	} else if r.Id == 4 {
		GraphLine = GraphLineFour
	} else if r.Id == 5 {
		GraphLine = GraphLineFive
	} else {
		GraphLine = 10
	}
	if r.Scroll == true {
		lc, err = linechart.New(
			linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
			linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
			linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
			linechart.XAxisUnscaled(),
		)
	} else {
		lc, err = linechart.New(
			linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(GraphAxes))),
			linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphYLabels))),
			linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(GraphXLabels))),
		)
	}
	inputs := r.Data.buffer
	if err != nil {
		fmt.Println("LineChart Error:", err)
	}
	//step1 = (step1 + 1) % len(inputs)
	if err := lc.Series("first", inputs,
		linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(GraphLine))),
	); err != nil {
		fmt.Println("LineChart Error:", err)
	}

	step := 0
	go periodic(ctx, time.Duration(10*time.Millisecond), func() error {
		defer func() {
			recover()
		}()
		var inputs []float64
		var inputLabels []string
		var averages []float64
		var graphWidth int

		if r.Scroll == true {
			graphWidth = lc.ValueCapacity()
		} else {
			//only scroll if we've reached 1million line
			graphWidth = 5000
		}
		inputs = r.Data.Last(graphWidth)
		inputLabels = r.Labels.Last(graphWidth)
		averages = r.Averages.Last(graphWidth)
		var labelMap = map[int]string{}
		for i, x := range inputLabels {
			labelMap[i] = x
		}

		if err := lc.Series("first", inputs,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(GraphLine))),
			linechart.SeriesXLabels(labelMap),
		); err != nil {
			return err
		}
		if r.Average == true {
			if step%10 == 1 {
				if err := lc.Series("average", averages,
					linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(LineHigh))),
					linechart.SeriesXLabels(labelMap),
				); err != nil {
					return err
				}
			}
		}

		step++
		return nil
	})
	return lc, err
}

func (r *Row) Update(x float64, dataLabel string, averageSeek int) {
	r.Data.Add(x)
	r.Labels.Add(dataLabel)
	r.DataContainer = append(r.DataContainer, x)
	r.AverageContainer = append(r.AverageContainer, findAverages(r.DataContainer))
	r.LabelContainer = append(r.LabelContainer, dataLabel)

	//find the average value of all values in Datacontainer
	avg := findAverages(r.Data.Last(averageSeek))
	r.Averages.Add(avg)
	if dataLabel != "" {
		r.Labels.Add(dataLabel)
		r.updateContainer(x, dataLabel)
	} else {
		r.Labels.Add("-")
		r.updateContainer(x, "-")
	}
}

func findAverages(values []float64) float64 {
	var total float64 = 0
	for _, value := range values {
		total += value
	}
	average := total / float64(len(values))
	return average
}

func (r *Row) updateContainer(value float64, label string) {
	defer func() {
		recover()
	}()
	r.DataContainer = append(r.DataContainer, value)
	r.AverageContainer = append(r.AverageContainer, findAverages(r.DataContainer))
	r.LabelContainer = append(r.LabelContainer, label)
}

//calulate data stats
func prepareStats(row *Row, buffer *float64RingBuffer) string {
	//use only accumulated data instead of ring buffer
	data := row.DataContainer
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
