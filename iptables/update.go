package iptables

import (
	"time"

	"github.com/pritunl/mongo-go-driver/bson"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/disk"

	"github.com/dropbox/godropbox/container/set"
	"github.com/pritunl/pritunl-cloud/firewall"
	"github.com/pritunl/pritunl-cloud/instance"
	"github.com/pritunl/pritunl-cloud/node"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/sirupsen/logrus"
)

type Update struct {
	OldState         *State
	NewState         *State
	Namespaces       []string
	FailedNamespaces set.Set
}

func (u *Update) Apply() {
	changed := false
	oldIfaces := set.NewSet()
	newIfaces := set.NewSet()

	namespacesSet := set.NewSet()
	for _, namespace := range u.Namespaces {
		namespacesSet.Add(namespace)
	}

	for iface := range u.OldState.Interfaces {
		oldIfaces.Add(iface)
	}
	for iface := range u.NewState.Interfaces {
		newIfaces.Add(iface)
	}

	oldIfaces.Subtract(newIfaces)
	for iface := range oldIfaces.Iter() {
		err := u.OldState.Interfaces[iface.(string)].Remove()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"iface": iface,
				"error": err,
			}).Error("iptables: Failed to delete removed interface iptables")
		}
	}

	for _, rules := range u.NewState.Interfaces {
		if u.FailedNamespaces.Contains(rules.Namespace) {
			logrus.WithFields(logrus.Fields{
				"namespace": rules.Namespace,
			}).Warn("iptables: Skipping failed namespace")
			continue
		}

		if rules.Namespace != "0" &&
			!namespacesSet.Contains(rules.Namespace) {

			_, err := utils.ExecCombinedOutputLogged(
				[]string{"File exists"},
				"ip", "netns",
				"add", rules.Namespace,
			)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"namespace": rules.Namespace,
					"error":     err,
				}).Error("iptables: Namespace add error")

				u.FailedNamespaces.Add(rules.Namespace)
				continue
			}
		}

		oldRules := u.OldState.Interfaces[rules.Namespace+"-"+rules.Interface]

		if (rules.Nat || rules.Nat6 || rules.OracleNat) &&
			(oldRules == nil || diffRulesNat(oldRules, rules)) {

			logrus.Info("iptables: Updating iptables nat")

			if oldRules != nil {
				err := oldRules.RemoveNat()
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"namespace": rules.Namespace,
						"error":     err,
					}).Error("iptables: Namespace remove nat error")

					u.FailedNamespaces.Add(rules.Namespace)
					continue
				}
			}

			err := rules.ApplyNat()
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"namespace": rules.Namespace,
					"error":     err,
				}).Error("iptables: Namespace apply nat error")

				u.FailedNamespaces.Add(rules.Namespace)
				continue
			}
		}

		if oldRules != nil {
			if !diffRules(oldRules, rules) {
				continue
			}

			if !changed {
				changed = true
				logrus.Info("iptables: Updating iptables")
			}

			if rules.Interface != "host" {
				err := rules.Hold()
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"namespace": rules.Namespace,
						"error":     err,
					}).Error("iptables: Namespace hold error")

					u.FailedNamespaces.Add(rules.Namespace)
					continue
				}
			}

			err := oldRules.Remove()
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"namespace": rules.Namespace,
					"error":     err,
				}).Error("iptables: Namespace remove error")

				u.FailedNamespaces.Add(rules.Namespace)
				continue
			}
		}

		if !changed {
			changed = true
			logrus.Info("iptables: Updating iptables")
		}

		err := rules.Apply()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"namespace": rules.Namespace,
				"error":     err,
			}).Error("iptables: Namespace apply error")

			u.FailedNamespaces.Add(rules.Namespace)
			continue
		}
	}

	return
}

func (u *Update) Recover() {
	if u.FailedNamespaces.Contains("0") {
		err := RecoverNode()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to recover node iptables, retrying")
			time.Sleep(10 * time.Second)
		}
	}

	if u.FailedNamespaces.Len() > 0 {
		logrus.Error("deploy: Failed to update iptables, " +
			"reloading state")

		time.Sleep(10 * time.Second)

		err := u.reload()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to recover iptables")
		}
	}
}

func (u *Update) reload() (err error) {
	db := database.GetDatabase()
	defer db.Close()

	namespaces, err := utils.GetNamespaces()
	if err != nil {
		return
	}

	disks, err := disk.GetNode(db, node.Self.Id)
	if err != nil {
		return
	}

	instances, err := instance.GetAllVirt(db, &bson.M{
		"node": node.Self.Id,
	}, disks)
	if err != nil {
		return
	}

	nodeFirewall, firewalls, err := firewall.GetAllIngress(
		db, node.Self, instances)
	if err != nil {
		return
	}

	err = Init(namespaces, instances, nodeFirewall, firewalls)
	if err != nil {
		return
	}

	return
}

func ApplyUpdate(newState *State, namespaces []string, recover bool) {
	lockId := stateLock.Lock()

	update := &Update{
		OldState:         curState,
		NewState:         newState,
		Namespaces:       namespaces,
		FailedNamespaces: set.NewSet(),
	}

	update.Apply()

	curState = newState

	stateLock.Unlock(lockId)

	if recover {
		update.Recover()
	}

	return
}

func UpdateState(nodeSelf *node.Node, instances []*instance.Instance,
	namespaces []string, nodeFirewall []*firewall.Rule,
	firewalls map[string][]*firewall.Rule) {

	newState := LoadState(nodeSelf, instances, nodeFirewall, firewalls)

	ApplyUpdate(newState, namespaces, false)

	return
}

func UpdateStateRecover(nodeSelf *node.Node, instances []*instance.Instance,
	namespaces []string, nodeFirewall []*firewall.Rule,
	firewalls map[string][]*firewall.Rule) {

	newState := LoadState(nodeSelf, instances, nodeFirewall, firewalls)

	ApplyUpdate(newState, namespaces, true)

	return
}
