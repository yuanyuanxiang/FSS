package server

type Executer interface {
	GetDeviceList() ([]map[string]interface{}, error)
	BlockDevice(serialNumber string) error
	AuthorizeDevice(serialNumber string) error
	IncreaseAllowance(key string, inc int)
	GetAuditLogs(typ string) ([]map[string]interface{}, error)
}

func NewExecuter() Executer {
	return &ExecuterImpl{}
}

// Send http request to the server and display the results.
type ExecuterImpl struct {
}

func (e *ExecuterImpl) GetDeviceList() ([]map[string]interface{}, error) {
	return nil, nil
}

func (e *ExecuterImpl) BlockDevice(serialNumber string) error {
	return nil
}

func (e *ExecuterImpl) AuthorizeDevice(serialNumber string) error {
	return nil
}

func (e *ExecuterImpl) IncreaseAllowance(key string, inc int) {
}

func (e *ExecuterImpl) GetAuditLogs(typ string) ([]map[string]interface{}, error) {
	return nil, nil
}
