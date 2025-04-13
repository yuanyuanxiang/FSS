# Firmware Security System

To prevent unauthorized manufacturing of our devices, we need to implement
a security system that ensures only authorized devices can receive firmware
updates. This system will prevent third-party manufacturers from producing
additional hardware units and updating them with our firmware.

## HTTP APIs

Server:

- GET /api/challenge/{serialNumber} - Generate and return a random challenge for device authentication
- POST /api/verify - Verify HMAC signature of the challenge and authorize device if allowance counter > 0
- POST /api/register - Register device public key with serial number after successful verification
- GET /api/firmware/{version} - Deliver signed firmware update to authenticated devices
- POST /api/update-allowance - Update the device registration allowance counter
- GET /api/devices - List all registered devices with their status
- GET /api/logs/updates - Retrieve logs of successful updates
- GET /api/logs/incidents - Retrieve logs of security incidents and rejected attempts
- POST /api/devices/{serialNumber}/block - Manually block a specific device
- POST /api/devices/{serialNumber}/authorize - Manually authorize a specific device

Simulator:

- POST /api/generate - Generate a batch of simulated devices
- POST /api/devices/{serialNumber}/request-update - Initiate update process for a specific device
- POST /api/devices/batch-update - Initiate update process for a batch of devices
- GET /api/devices/{serialNumber} - Get status information for a specific device
- POST /api/simulate/replay/{serialNumber} - Simulate a replay attack with a specific serial number
- GET /api/devices/status - Get status of all simulated devices

## Command-Line interfaces

Server:

- server --port=`port` - Start the server on specified port
- server --allowance=`number` - Set initial device registration allowance
- server --increase-allowance=`number` - Increase allowance counter by specified amount
- server --list-devices - Display all registered devices
- server --show-incidents - Display security incident logs
- server --show-updates - Display successful update logs
- server --block=`serialNumber` - Block a specific device
- server --authorize=`serialNumber` - Authorize a specific device

Simulator:

- simulator --generate=`count` --start-serial=`number` - Generate specified number of devices
- simulator --update=`serialNumber` - Request update for a specific device
- simulator --batch-update=`startSerial`-`endSerial` - Request updates for a range of devices
- simulator --status=`serialNumber` - Show status of a specific device
- simulator --list-all - List all simulated devices with their status
- simulator --simulate-replay=`serialNumber` - Simulate a replay attack
- simulator --simulate-batch-

## Build the program

Run `cmd` under `FSS\cmd\fss` and execute `go build .` Also, execute `make` to build the traget program.

Then we'll get `FSS.exe` on Windows system.

## Run the program

Run `FSS.exe server --port=9000 --allowance=100` with `cmd` and we will start the server program.

Run `FSS.exe simulator --port=9001` with `cmd` and we will start the device simulator.

There is a configuration file `apis.json` for server and simulator. Each HTTP request is defined in it.

## Test the program

Under folder `test` there is a Postman script. Use it to send command to simulator. We can also test the program with
command-line interfaces.
