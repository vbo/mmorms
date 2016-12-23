trap 'kill %1' SIGINT
../../bin/overlord --addr=localhost:7070 & ../../bin/mmorms --addr=localhost:8080 --overlord=localhost:7070
