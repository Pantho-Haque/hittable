package screens

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/noderuntime"
	"github.com/hittable/shellapp/ui/components/extpanel"
)

func extpanelActions(m *MainScreen) extpanel.Actions {
	return extpanel.Actions{
		Install: func(spec string) { m.pendingCmd = m.extInstall(spec) },
		Uninstall: func(id string) {
			if err := extensions.Uninstall(id); err != nil {
				m.extNote("error", err.Error())
				return
			}
			m.extNote("info", "Uninstalled "+id)
			m.ExtPanel.Reload()
			m.pendingCmd = m.extRestart()
		},
		SetEnabled: func(id string, on bool) {
			if err := extensions.SetEnabled(id, on); err != nil {
				m.extNote("error", err.Error())
				return
			}
			verb := "Disabled "
			if on {
				verb = "Enabled "
			}
			m.extNote("info", verb+id)
			m.ExtPanel.Reload()
			m.pendingCmd = m.extRestart()
		},
		InstallNode: func() { m.pendingCmd = m.extInstallNodeNow(nil) },
		Trust:       func() { m.pendingCmd = m.cmdTrust() },
		SetTheme: func(extID string) {
			var mine []extensions.ThemeChoice
			for _, t := range extensions.Themes() {
				if t.ExtID == extID {
					mine = append(mine, t)
				}
			}
			if len(mine) == 1 {
				if err := m.writeSetting("workbench.colorTheme", mine[0].Label, "user"); err != nil {
					m.extNote("error", err.Error())
					return
				}
				m.extApplyTheme()
				m.extNote("info", "Color theme: "+mine[0].Label)
				return
			}
			m.cmdTheme()
		},
		RestartHost: func() { m.Ext.crashes = nil; m.pendingCmd = m.extRestart() },
		ShowLog:     func() { m.ExtOpen = false; m.showOutput("Extension Host") },
		ShowNodeLog: func() { m.ExtOpen = false; m.showOutput("Node.js install") },
		State: func() extpanel.State {
			x := m.Ext
			pct := x.installPct
			if x.installing == "" {
				pct = -1
			}
			st := extpanel.State{
				Installing: x.installing, Progress: pct, Spinner: m.Spinner.View(),
				Trusted: extensions.Trusted(m.RootDir), Activation: x.activation,
				NodeBusy: strings.Contains(x.installing, "Node.js"), NodeLine: x.nodeLine, NodeErr: x.nodeErr,
			}
			if f, err := noderuntime.Find(); err == nil {
				st.NodeOK, st.NodeVersion, st.NodeWhere = true, f.Version, f.Source+" · "+f.Path
			} else {
				st.NodeProblem = "Needs Node.js " + fmt.Sprint(noderuntime.MinMajor) + "+, and none is installed"
				if errors.Is(err, noderuntime.ErrTooOld) {
					st.NodeProblem = err.Error()
				}
				if plan, perr := noderuntime.PlanInstall(); perr == nil {
					st.NodePlan = plan.Summary
				} else {
					st.NodePlan = perr.Error()
				}
			}
			switch {
			case x.host != nil:
				st.Host = "running"
			case x.starting:
				st.Host = "starting"
			case x.stopped == "crashed" || x.stopped == "failed":
				st.Host, st.HostNote = "stopped", "it crashed; see the log"
			}
			return st
		},
	}
}
