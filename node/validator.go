// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2024 Pete Heist

package node

// A validater can perform validation on itself.  Any runner that also
// implements validater will be validated after the config is parsed.
type validater interface {
	validate() error
}
