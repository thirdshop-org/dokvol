package system

type System struct {
	Drives       []DriveInfo
	Applications []Application
}

func New() System {

	drives := GetDrives()

	return System{
		Drives:       drives,
		Applications: GetApplicationsDetails(drives),
	}
}
