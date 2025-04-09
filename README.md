# Firmware Security System

To prevent unauthorized manufacturing of our devices, we need to implement
a security system that ensures only authorized devices can receive firmware
updates. This system will prevent third-party manufacturers from producing
additional hardware units and updating them with our firmware.

## Build the program

Run `cmd` under `FSS\cmd\fss` and execute `go build .`

Then we'll get `FSS.exe` on Windows system.

## Run the program

Run `FSS.exe server --port=9000 --allowance=100` with `cmd` and we will start the server program.

Run `FSS.exe simulator --port=9001` with `cmd` and we will start the device simulator.

There is a configuration file `apis.json` for server and simulator. Each HTTP request is defined in it.

## Test the program

Under folder `test` there is a Postman script. Use it to send command to simulator.
