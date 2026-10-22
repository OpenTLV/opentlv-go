// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package main

import (
	"fmt"

	"github.com/OpenTLV/opentlv-go"
)

func main() {
	fmt.Println("Go binding:", opentlv.BindingVersion)
	fmt.Println("Linked Core:", opentlv.Version())
}
