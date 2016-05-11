package restful

// Copyright 2016 Ernest Micklei. All rights reserved.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

import "sync/atomic"

type CompressorCacheStats struct {
	GzipWriterHits   int64
	GzipWriterMisses int64
	GzipReaderHits   int64
	GzipReaderMisses int64
	ZlibWriterHits   int64
	ZlibWriterMisses int64
}

func (b CompressorCacheStats) HitGzipWriter(isHit bool) {
	if isHit {
		atomic.AddInt64(&b.GzipWriterHits, 1)
	} else {
		atomic.AddInt64(&b.GzipWriterMisses, 1)
	}
}

func (b CompressorCacheStats) HitGzipReader(isHit bool) {
	if isHit {
		atomic.AddInt64(&b.GzipReaderHits, 1)
	} else {
		atomic.AddInt64(&b.GzipReaderMisses, 1)
	}
}
func (b CompressorCacheStats) HitZlibWriter(isHit bool) {
	if isHit {
		atomic.AddInt64(&b.ZlibWriterHits, 1)
	} else {
		atomic.AddInt64(&b.ZlibWriterMisses, 1)
	}
}
