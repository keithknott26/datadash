package datadash

type Buffer struct {
	Data       *float64RingBuffer
	DataLabels *stringRingBuffer
}

func NewBuffer(bufsize int) *Buffer {
	lc := &Buffer{
		Data:       newFloat64RingBuffer(bufsize),
		DataLabels: newStringRingBuffer(bufsize),
	}
	return lc
}

//reshaping an array:
//s := []float64{0, 1, 2, 3, 4, 5, 6, 7}
//t := reshape(s, [2]int{4,2})
//fmt.Println(t[2,0]) // prints 4
//t[1,0] = -2
//t2 := reshape(s, [...]int{2,2,2})
//fmt.Println(t3[0,1,0]) // prints -2
//t3 := reshape(s, [...]int{2,2,2,2}) // runtime panic: reshape length mismatch

//int array2d[][] = new int[10][3];
//for(int i=0; i<10;i++)
//for(int j=0;j<3;j++)
//	array2d[i][j] = array1d[(j*10) + i];

func (lc *Buffer) Buffer() []float64 {
	defer func() {
		recover()
	}()
	return lc.Data.buffer
}

func (lc *Buffer) LastLabels(height int, width int) []string {
	defer func() {
		recover()
	}()
	return lc.DataLabels.Last(width)
}
func (lc *Buffer) Last(height int, width int) []float64 {
	defer func() {
		recover()
	}()
	return lc.Data.Last(width)
}
func (lc *Buffer) Add(x float64, dataLabel string) {
	//fmt.Println("Buffer Add Value:", x)
	lc.Data.Add(x)
	//fmt.Println("Buffer Add Value Done:")
	//fmt.Println("Buffer DataLabel Value:", dataLabel)
	if dataLabel != "" {
		//fmt.Println("Buffer Label Value:", dataLabel)
		lc.DataLabels.Add(dataLabel)
		//fmt.Println("Buffer Label Value Added:", dataLabel)
	}
	lc.updatePlotData()
}

func (lc *Buffer) Update(xs []float64, dataLabels []string) {
	lc.Data = newFloat64RingBuffer(lc.Data.Capacity())
	if xs != nil {
		for _, x := range xs {
			lc.Data.Add(x)
		}
	}
	lc.DataLabels = newStringRingBuffer(lc.DataLabels.Capacity())
	if dataLabels != nil {
		for _, dataLabel := range dataLabels {
			lc.DataLabels.Add(dataLabel)
		}
	}
}

func (lc *Buffer) Clear() {
	lc.Update(nil, nil)
}

func (lc *Buffer) updatePlotData() {
	defer func() {
		recover()
	}()
	//if lc.Mode == "dot" {
	//data := lc.Data.Last(2 * 100)
	//lc.Plot.Data[0] = data
	//labels := lc.DataLabels.Last(2 * 100)
	//lc.Plot.DataLabels = labels
	//} else {
	//data := lc.Data.Last(100)
	//lc.Plot.Data[0] = data
	//labels := lc.DataLabels.Last(100)
	//lc.Plot.DataLabels = labels
	//}
	////lc.Plot.Border.Label = "Left Arrow to Pause - Right Arrow to Resume"

}
