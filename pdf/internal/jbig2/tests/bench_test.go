/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package tests

import (
	"archive/zip"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/unidoc/unidoc/pdf/internal/jbig2"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkDecodeSingleJBIG2 benchmarks the jbig2 decoding
// in order to run the benchmark run the DecodeJBIG2Files with the same JBIG2 environmental variable
// the zipped files containing raw jbig2 streams shoud be created
func BenchmarkDecodeSingleJBIG2(b *testing.B) {
	b.Helper()
	dirName := os.Getenv("JBIG2")
	require.NotEmpty(b, dirName, "No Environment variable 'JBIG2' found")

	jbig2Files, err := readJBIGZippedFiles(dirName)
	require.NoError(b, err)

	for _, file := range jbig2Files {

		zr, err := zip.OpenReader(filepath.Join(dirName, file))
		require.NoError(b, err)

		defer zr.Close()

		for _, zFile := range zr.File {
			sf, err := zFile.Open()
			require.NoError(b, err)

			defer sf.Close()

			data, err := ioutil.ReadAll(sf)
			require.NoError(b, err)

			b.Run(fmt.Sprintf("%s/%d", zFile.Name, len(data)), func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					d, err := jbig2.NewDocument(data)
					require.NoError(b, err)

					p, err := d.GetPage(1)
					require.NoError(b, err)

					_, err = p.GetBitmap()
					require.NoError(b, err)
				}
			})
		}
	}

}
