package vmware

import (
	"fmt"
	"context"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vim25/methods"
)

func initializeHostClient(ctx context.Context, hostIp string, userName string, userPassword string) (client *govmomi.Client, hostSystem *object.HostSystem, err error) {
	var dc *object.Datacenter
	var hosts []*object.HostSystem
	sdkURL := &url.URL{
		Scheme: "https",
		Host:   hostIp,
		Path:   "/sdk",
	}
	sdkURL.User = url.UserPassword(userName, userPassword)
	if client, err = govmomi.NewClient(ctx, sdkURL, true); err != nil {
		err = fmt.Errorf("failure to connect to the host %s: %w", hostIp, err)
		return
	}
	finder := find.NewFinder(client.Client, true)
	if dc, err = finder.DefaultDatacenter(ctx); err != nil {
		err = fmt.Errorf("failure to find default datacenter: %w", err)
		return
        }
	finder.SetDatacenter(dc)
	if hosts, err = finder.HostSystemList(ctx, "*"); err != nil || len(hosts) == 0 {
		err = fmt.Errorf("failure to find host: %w", err)
		return
        }
	hostSystem = hosts[0]
	return
}

func getHostState(ctx context.Context, hostSystem *object.HostSystem) (state string, err error) {
	var props mo.HostSystem
	if err = hostSystem.Properties(ctx, hostSystem.Reference(), []string{"runtime.connectionState"}, &props); err != nil {
		err = fmt.Errorf("failure to get host properties: %w", err)
	} else {
		state = string(props.Runtime.ConnectionState)
	}
	return
}

func isInMaintenanceMode(ctx context.Context, hostSystem *object.HostSystem) (inMaintenanceMode bool, err error) {
	var props mo.HostSystem
	if err = hostSystem.Properties(ctx, hostSystem.Reference(), []string{"runtime.inMaintenanceMode"}, &props); err != nil {
		err = fmt.Errorf("failure to get maintenance mode status: %w", err)
	} else {
		inMaintenanceMode = props.Runtime.InMaintenanceMode
	}
	return
}

func enterMaintainanceMode(ctx context.Context, hostSystem *object.HostSystem, timeout int) (err error) {
	var inMaintenanceMode bool
	var task *object.Task
	if inMaintenanceMode, err = isInMaintenanceMode(ctx, hostSystem); err != nil {
		return
	}
	if inMaintenanceMode {
		return
	}
	if task, err = hostSystem.EnterMaintenanceMode(ctx, int32(timeout), true, nil); err != nil {
		err = fmt.Errorf("failure to enter maintenance mode: %w", err)
		return
	}
	if err = task.Wait(ctx); err != nil {
		err =  fmt.Errorf("enter maintenance mode task failed: %w", err)
	}
	return
}

func exitMaintainanceMode(ctx context.Context, hostSystem *object.HostSystem, timeout int) (err error) {
	var inMaintenanceMode bool
	var task *object.Task
	if inMaintenanceMode, err = isInMaintenanceMode(ctx, hostSystem); err != nil {
		return
	}
	if !inMaintenanceMode {
		return
	}
	if task, err = hostSystem.ExitMaintenanceMode(ctx, int32(timeout)); err != nil {
		err = fmt.Errorf("failure to exit maintenance mode: %w", err)
		return
	}
	if err = task.Wait(ctx); err != nil {
		err =  fmt.Errorf("exit maintenance mode task failed: %w", err)
	}
	return
}

func shutdownHost(ctx context.Context, hostSystem *object.HostSystem) (err error) {
	var res *types.ShutdownHost_TaskResponse
	var inMaintenanceMode bool
	if inMaintenanceMode, err = isInMaintenanceMode(ctx, hostSystem); err != nil {
		return
	}
	if !inMaintenanceMode {
		err = fmt.Errorf("host is not in maintenance mode")
		return
	}
	req := &types.ShutdownHost_Task{
		This:  hostSystem.Reference(),
		Force: false,
	}
	if res, err = methods.ShutdownHost_Task(ctx, hostSystem.Client(), req); err != nil {
		err = fmt.Errorf("failure to shutdown host: %w", err)
		return
        }
	task := object.NewTask(hostSystem.Client(), res.Returnval)
	if err = task.Wait(ctx); err != nil {
		err = fmt.Errorf("shutdown task failed: %w", err)
        }
	return
}
