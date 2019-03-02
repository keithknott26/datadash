# dataDash

<script id="asciicast-BjSD4WDbIYH2DDH3p2kcIy77L" src="https://asciinema.org/a/BjSD4WDbIYH2DDH3p2kcIy77L.js" async></script>

[![asciicast](https://asciinema.org/a/BjSD4WDbIYH2DDH3p2kcIy77L.svg)](https://asciinema.org/a/BjSD4WDbIYH2DDH3p2kcIy77L)

<a href="https://asciinema.org/a/BjSD4WDbIYH2DDH3p2kcIy77L" target="_blank"><img src="https://asciinema.org/a/BjSD4WDbIYH2DDH3p2kcIy77L.svg" /></a>

Demo 5 Graphs (6 columns of data):
seq 4000 | awk 'BEGIN{OFS="\t"; print "x","sin(x)","cos(x)", "rand(x)", "rand(x)", "rand(x)"}{x=$1/10; print x,sin(x),cos(x),rand(x),rand(x),rand(x); system("sleep 0.02")}'  | ./datadash

Demo 1 Graph (2 columns of data):
 seq 4000 | awk 'BEGIN{OFS="\t"; print "x","sin(x)"}{x=$1/10; print x,sin(x); system("sleep 0.02")}'  | ./datadash --label-mode time

 Demo 1 Graph (1 column of streaming data):
  seq 4000 | awk 'BEGIN{OFS="\t"; print "x"}{x=$1/10; print x system("sleep 0.02")}'  | ./datadash --label-mode time
