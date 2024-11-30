// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2024 Pete Heist

package antler

import (
	_ "embed"
)

// styleTemplate is the template for global CSS styles.
//
//go:embed style.css.tmpl
var styleTemplate string
