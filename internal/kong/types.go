/*
 * Copyright (c) 2022 by Bank Lombard Odier & Co Ltd, Geneva, Switzerland. This software is subject
 * to copyright protection under the laws of Switzerland and other countries. ALL RIGHTS RESERVED.
 *
 */

package kong

// File represents a File in Kong.
type File struct {
	Checksum  *string `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	ID        *string `json:"id,omitempty" yaml:"id,omitempty"`
	CreatedAt *int    `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	Path      *string `json:"path,omitempty" yaml:"path,omitempty"`
	Contents  *string `json:"contents,omitempty" yaml:"contents,omitempty"`
}
