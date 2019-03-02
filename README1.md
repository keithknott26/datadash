# dataDash
Visualize data inside the terminal

## Description

###### A graphing application written in Go using termdash UI libraries, inspired by termeter. Delimited Data can be passed in by pipe or directly from a file.

[![asciicast](https://asciinema.org/a/BjSD4WDbIYH2DDH3p2kcIy77L.svg)](https://asciinema.org/a/BjSD4WDbIYH2DDH3p2kcIy77L)

###### Demo (6 columns of data):
```bash
$ seq 4000 | awk 'BEGIN{OFS="\t"; print "x","sin(x)","cos(x)", "rand(x)", "rand(x)", "rand(x)"}{x=$1/10; print x,sin(x),cos(x),rand(x),rand(x),rand(x); system("sleep 0.02")}'  | ./datadash
```

###### Demo (2 columns of data):
 ```bash
$ seq 4000 | awk 'BEGIN{OFS="\t"; print "x","sin(x)"}{x=$1/10; print x,sin(x); system("sleep 0.02")}'  | ./datadash --label-mode time
```

###### Demo (Streaming data):
```bash
 $ seq 4000 | awk 'BEGIN{OFS="\t"; print "x"}{x=$1/10; print x system("sleep 0.02")}'  | ./datadash --label-mode time
```

### Installation
```bash
$ go get -u github.com/keithknott26/datadash
$ go build $GOPATH/github.com/keithknott26/datadash/cmd/datadash.go
```
datadash can accept tabular data like CSV, TSV, or you can use a custom delimiter with the -d option. The default delimiter is tab.

## Arguments

```bash
$ usage: datadash [<flags>] [<input file>]

A Data Visualization Tool

Flags:
      --help                  Show context-sensitive help (also try --help-long and --help-man).
      --debug                 Enable Debug Mode
  -d, --delimiter="\t"        Record Delimiter:
  -m, --label-mode="first"    X-Axis Labels: 'first' (use the first record in the column) or 'time' (use the current time)
  -s, --scroll                Whether or not to scroll chart data
  -a, --average-line          Enables the line representing the average of values
  -z, --average-seek=500      The number of values to consider when displaying the average line: (50,100,500...)
  -r, --redraw-interval=10ms  The interval at which objects on the screen are redrawn: (100ms,250ms,1s,5s..)
  -l, --seek-interval=20ms    The interval at which records (lines) are read from the datasource: (100ms,250ms,1s,5s..)

Args:
  [<input file>]  A file containing a label header, and data in columns separated by delimiter 'd'. Data piped from Stdin uses the same format
```

## Chart types
datadash currently supports following chart types:

* Line
  * Plot data as line graph
  * Line graph supports zooming
  * Supports scrolling for streaming data applications
  * Scrolling can be disabled with the --no-scroll option