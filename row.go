package datadash

import (
	"context"
	"fmt"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/sparkline"
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
	scrolling = iota
	line
	bar
)

// sort mode of counter panel
const (
	sort_none = iota
	sort_alphabetical
	sort_numeric
)

var (
	parOneBorder     = 26
	parOneTitle      = 82
	parTwoBorder     = 25
	parTwoTitle      = 13
	parThreeBorder   = 25
	parThreeTitle    = 45
	parFourBorder    = 25
	parFourTitle     = 9
	parFiveTitle     = 165
	parFiveBorder    = 25
	parText          = 3
	parPointer       = 248
	parValue         = 15
	graphOneBorder   = 25
	graphTwoBorder   = 25
	graphThreeBorder = 25
	graphFourBorder  = 25
	graphFiveBorder  = 25
	graphAxes        = 8
	graphXLabels     = 248
	graphYLabels     = 15
	graphTitles      = 2
	lineLow          = 235
	lineAvg          = 235
	lineHigh         = 239
	graphLineOne     = 82
	graphLineTwo     = 13
	graphLineThree   = 45
	graphLineFour    = 9
	graphLineFive    = 165
	graphLinePaused  = 1
	ctx              context.Context
)

//type RowInterface interface {
//	NewRow()      // same as adding the methods of ReadWriter
//	InitWidgets() // same as adding the methods of Locker
//	returnChart()
//}

type Row struct {
	ID               int
	Label            string
	Scroll           bool
	Average          bool
	Labels           *stringRingBuffer
	Context          context.Context
	Data             *float64RingBuffer
	Averages         *float64RingBuffer
	LineChart        *linechart.LineChart
	YAxisAdaptive    bool
	BarChart         *barchart.BarChart
	SparkLine        *sparkline.SparkLine
	Textbox          *text.Text
	DataContainer    []float64
	LabelContainer   []string
	AverageContainer []float64
	RedrawInterval   time.Duration
	SeekInterval     time.Duration
}

//func (self *Row) increment() {
//	self.id++
//}

func NewRow(ctx context.Context, label string, bufsize int, id int, scroll bool, average bool, yAxisAdaptive bool) *Row {
	row := &Row{
		ID:            id,
		Scroll:        scroll,
		YAxisAdaptive: yAxisAdaptive,
		Average:       average,
		Label:         label,
		Context:       ctx,
		Data:          newFloat64RingBuffer(bufsize),
		Labels:        newStringRingBuffer(bufsize),
		Averages:      newFloat64RingBuffer(bufsize),
	}
	return row
}

func (r *Row) InitWidgets(ctx context.Context, graphType string, label string, reDrawInterval time.Duration, seekInterval time.Duration) *Row {
	r.Label = label
	r.RedrawInterval = reDrawInterval
	r.SeekInterval = seekInterval
	r.Textbox = r.newTextBox(ctx, label)
	r.LineChart = r.newLineChart(ctx)

	if graphType == "bar" {
		r.BarChart = r.newBarChart(ctx)
	}
	if graphType == "spark" {
		r.SparkLine = r.newSparkLine(ctx)
	}
	return r
}

func (r *Row) newLineChart(ctx context.Context) *linechart.LineChart {
	lc, err := r.createLineChart(ctx)
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

func (r *Row) newBarChart(ctx context.Context) *barchart.BarChart {
	bc, err := r.createBarGraph(ctx)
	if err != nil {
		fmt.Println("BarGraph Error:", err)
	}
	return bc
}

func (r *Row) newSparkLine(ctx context.Context) *sparkline.SparkLine {
	bc, err := r.createSparkLine(ctx)
	if err != nil {
		fmt.Println("Sparkline Error:", err)
	}
	return bc
}

func (r *Row) ContainerOptions(ctx context.Context, graphType string) []container.Option {
	var ParBorder int
	var GraphBorder int
	var row []container.Option
	switch r.ID {
	case 0:
		GraphBorder = graphOneBorder
		ParBorder = parOneBorder
	case 1:
		GraphBorder = graphOneBorder
		ParBorder = parOneBorder
	case 2:
		GraphBorder = graphTwoBorder
		ParBorder = parTwoBorder
	case 3:
		GraphBorder = graphThreeBorder
		ParBorder = parThreeBorder
	case 4:
		GraphBorder = graphFourBorder
		ParBorder = parFourBorder
	case 5:
		GraphBorder = graphFiveBorder
		ParBorder = parFiveBorder
	}
	switch graphType {
	case "line":
		row = []container.Option{
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
					container.BorderTitle(r.Label+" - 'q' Quit | 'p' Pause 10s | <- Slow | Resume -> | Scroll to Zoom..."),
					container.BorderColor(cell.ColorNumber(GraphBorder)),
					container.PlaceWidget(r.LineChart),
				),
				container.SplitPercent(15),
			)}
	case "bar":
		row = []container.Option{
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
					container.BorderTitle(r.Label+" - 'q' Quit | 'p' Pause 10s | <- Slow | Resume -> | Scroll to Zoom..."),
					container.BorderColor(cell.ColorNumber(GraphBorder)),
					container.PlaceWidget(r.BarChart),
				),
				container.SplitPercent(15),
			)}
	case "spark":
		row = []container.Option{
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
					container.BorderTitle(r.Label+" - 'q' Quit | 'p' Pause 10s | <- Slow | Resume -> | Scroll to Zoom..."),
					container.BorderColor(cell.ColorNumber(GraphBorder)),
					container.PlaceWidget(r.SparkLine),
				),
				container.SplitPercent(15),
			)}
	default:
		row = []container.Option{
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
					container.BorderTitle(r.Label+" - 'q' Quit | 'p' Pause 10s | <- Slow | Resume -> | Scroll to Zoom..."),
					container.BorderColor(cell.ColorNumber(GraphBorder)),
					container.PlaceWidget(r.LineChart),
				),
				container.SplitPercent(15),
			)}
	}
	return row
}

func (r *Row) newText(ctx context.Context, label string) (*text.Text, error) {
	var ParTitle int
	switch r.ID {
	case 0:
		ParTitle = parOneTitle
	case 1:
		ParTitle = parOneTitle
	case 2:
		ParTitle = parTwoTitle
	case 3:
		ParTitle = parThreeTitle
	case 4:
		ParTitle = parFourTitle
	case 5:
		ParTitle = parFiveTitle
	}

	t, err := text.New()
	context := ctx
	go periodic(context, r.RedrawInterval/2, func() error {
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
		if err := t.Write(fmt.Sprintf("\nTime:        %s", pointer), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(parPointer)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("\nValue:       %.2f", value), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(parValue)))); err != nil {
			return err
		}
		if err := t.Write(fmt.Sprintf("%s", data), text.WriteCellOpts(cell.FgColor(cell.ColorNumber(parText)))); err != nil {
			return err
		}
		return nil
	})
	return t, err
}
func (r *Row) createBarGraph(ctx context.Context) (*barchart.BarChart, error) {
	var ParTitle int
	switch r.ID {
	case 0:
		ParTitle = parOneTitle
	case 1:
		ParTitle = parOneTitle
	case 2:
		ParTitle = parTwoTitle
	case 3:
		ParTitle = parThreeTitle
	case 4:
		ParTitle = parFourTitle
	case 5:
		ParTitle = parFiveTitle
	}
	barcolors := make([]cell.Color, 0, 0)
	valuecolors := make([]cell.Color, 0, 0)
	for i := 1; i <= 100; i++ {
		barcolors = append(barcolors, cell.ColorNumber(ParTitle))
	}
	for i := 1; i <= 100; i++ {
		valuecolors = append(valuecolors, cell.ColorBlack)
	}
	bc, err := barchart.New(
		barchart.BarColors(barcolors),
		barchart.ValueColors(valuecolors),
		barchart.ShowValues(),
		barchart.BarWidth(3),
	)

	if err != nil {
		return nil, err
	}
	go periodic(ctx, r.RedrawInterval, func() error {
		defer func() {
			recover()
		}()
		var inputs []float64

		inputs = r.Data.Last(bc.ValueCapacity())
		values := make([]int, 0)
		//use averages instead //TODO
		if r.Average == true {
			//averages
			inputs = r.Averages.Last(bc.ValueCapacity())
		}
		for _, x := range inputs {
			values = append(values, round(x))
		}
		max := values[0] // assume first value is the smallest
		for _, value := range values {
			if value > max {
				max = value // found another smaller value, replace previous value in max
			}
		}

		return bc.Values(values, max+1)
	})
	return bc, err

}

func (r *Row) createSparkLine(ctx context.Context) (*sparkline.SparkLine, error) {
	var ParTitle int
	switch r.ID {
	case 0:
		ParTitle = parOneTitle
	case 1:
		ParTitle = parOneTitle
	case 2:
		ParTitle = parTwoTitle
	case 3:
		ParTitle = parThreeTitle
	case 4:
		ParTitle = parFourTitle
	case 5:
		ParTitle = parFiveTitle
	}

	sl, err := sparkline.New(
		sparkline.Color(cell.Color(ParTitle)),
	)
	if err != nil {
		panic(err)
	}
	go periodic(ctx, r.RedrawInterval*4, func() error {
		defer func() {
			recover()
		}()
		var inputs []float64

		inputs = r.Data.Last(1)
		values := make([]int, 0)
		//use averages instead //TODO
		if r.Average == true {
			//averages
			inputs = r.Averages.Last(sl.ValueCapacity())
		}
		for _, x := range inputs {
			// display only positive numbers since this is required by sparkline
			if round(x) > 0 {
				values = append(values, round(x))
			}
		}
		max := values[0] // assume first value is the smallest
		for _, value := range values {
			if value > max {
				max = value // found another smaller value, replace previous value in max
			}
		}
		return sl.Add(values)
	})
	return sl, err

}

func (r *Row) createLineChart(ctx context.Context) (*linechart.LineChart, error) {
	//set the line color based on the r.ID
	var GraphLine int
	var lc *linechart.LineChart
	var err error

	switch r.ID {
	case 0:
		GraphLine = graphLineOne
	case 1:
		GraphLine = graphLineOne
	case 2:
		GraphLine = graphLineTwo
	case 3:
		GraphLine = graphLineThree
	case 4:
		GraphLine = graphLineFour
	case 5:
		GraphLine = graphLineFive
	default:
		GraphLine = 10
	}

	if r.Scroll == true {
		if r.YAxisAdaptive == true {
			lc, err = linechart.New(
				linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(graphAxes))),
				linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(graphYLabels))),
				linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(graphXLabels))),
				linechart.XAxisUnscaled(),
				linechart.YAxisAdaptive(),
			)
		} else {
			lc, err = linechart.New(
				linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(graphAxes))),
				linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(graphYLabels))),
				linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(graphXLabels))),
				linechart.XAxisUnscaled(),
			)
		}
	} else {
		if r.YAxisAdaptive == true {
			lc, err = linechart.New(
				linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(graphAxes))),
				linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(graphYLabels))),
				linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(graphXLabels))),
				linechart.YAxisAdaptive(),
			)
		} else {
			lc, err = linechart.New(
				linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(graphAxes))),
				linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(graphYLabels))),
				linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(graphXLabels))),
			)
		}
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
	go periodic(ctx, time.Duration(r.RedrawInterval), func() error {
		defer func() {
			recover()
		}()
		var inputs []float64
		var inputLabels []string
		var averages []float64
		var graphWidth int

		if r.Scroll == true {
			graphWidth = lc.ValueCapacity()
			inputs = r.Data.Last(graphWidth)
			inputLabels = r.Labels.Last(graphWidth)
			averages = r.Averages.Last(graphWidth)
		} else {
			//only scroll if we've reached 5000 records
			inputs = r.DataContainer
			inputLabels = r.LabelContainer
			averages = r.AverageContainer
		}
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
					linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(lineHigh))),
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
	//add values to ring buffer and data containers
	r.Data.Add(x)
	r.Labels.Add(dataLabel)
	r.DataContainer = append(r.DataContainer, x)
	r.LabelContainer = append(r.LabelContainer, dataLabel)
	r.AverageContainer = append(r.AverageContainer, findAverages(r.DataContainer))

	//find the average value of all values in Datacontainer
	avg := findAverages(r.Data.Last(averageSeek))
	r.Averages.Add(avg)
}

func findAverages(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	average := total / float64(len(values))
	return average
}

// calulate data stats
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
				// panic(err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// rounding functions used by the bar chart
func round(val float64) int {
	if val < 0 {
		return int(val - 0.5)
	}
	return int(val + 0.5)
}

func roundUp(val float64) int {
	if val > 0 {
		return int(val + 1.0)
	}
	return int(val)
}
