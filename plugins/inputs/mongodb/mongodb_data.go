package mongodb

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/shanas-swi/telegraf-v1.16.3"
)

type MongodbData struct {
	StatLine      *StatLine
	Fields        map[string]interface{}
	Tags          map[string]string
	DbData        []DbData
	ColData       []ColData
	ShardHostData []DbData
}

type DbData struct {
	Name   string
	Fields map[string]interface{}
}

type ColData struct {
	Name   string
	DbName string
	Fields map[string]interface{}
}

func NewMongodbData(statLine *StatLine, tags map[string]string) *MongodbData {
	return &MongodbData{
		StatLine: statLine,
		Tags:     tags,
		Fields:   make(map[string]interface{}),
		DbData:   []DbData{},
	}
}

var DefaultStats = map[string]string{
	"uptime_ns":                 "UptimeNanos",
	"inserts":                   "InsertCnt",
	"inserts_per_sec":           "Insert",
	"queries":                   "QueryCnt",
	"queries_per_sec":           "Query",
	"updates":                   "UpdateCnt",
	"updates_per_sec":           "Update",
	"deletes":                   "DeleteCnt",
	"deletes_per_sec":           "Delete",
	"getmores":                  "GetMoreCnt",
	"getmores_per_sec":          "GetMore",
	"commands":                  "CommandCnt",
	"commands_per_sec":          "Command",
	"flushes":                   "FlushesCnt",
	"flushes_per_sec":           "Flushes",
	"flushes_total_time_ns":     "FlushesTotalTime",
	"vsize_megabytes":           "Virtual",
	"resident_megabytes":        "Resident",
	"queued_reads":              "QueuedReaders",
	"queued_writes":             "QueuedWriters",
	"active_reads":              "ActiveReaders",
	"active_writes":             "ActiveWriters",
	"available_reads":           "AvailableReaders",
	"available_writes":          "AvailableWriters",
	"total_tickets_reads":       "TotalTicketsReaders",
	"total_tickets_writes":      "TotalTicketsWriters",
	"net_in_bytes_count":        "NetInCnt",
	"net_in_bytes":              "NetIn",
	"net_out_bytes_count":       "NetOutCnt",
	"net_out_bytes":             "NetOut",
	"open_connections":          "NumConnections",
	"ttl_deletes":               "DeletedDocumentsCnt",
	"ttl_deletes_per_sec":       "DeletedDocuments",
	"ttl_passes":                "PassesCnt",
	"ttl_passes_per_sec":        "Passes",
	"cursor_timed_out":          "TimedOutC",
	"cursor_timed_out_count":    "TimedOutCCnt",
	"cursor_no_timeout":         "NoTimeoutC",
	"cursor_no_timeout_count":   "NoTimeoutCCnt",
	"cursor_pinned":             "PinnedC",
	"cursor_pinned_count":       "PinnedCCnt",
	"cursor_total":              "TotalC",
	"cursor_total_count":        "TotalCCnt",
	"document_deleted":          "DeletedD",
	"document_inserted":         "InsertedD",
	"document_returned":         "ReturnedD",
	"document_updated":          "UpdatedD",
	"connections_current":       "CurrentC",
	"connections_available":     "AvailableC",
	"connections_total_created": "TotalCreatedC",
	"operation_scan_and_order":  "ScanAndOrderOp",
	"operation_write_conflicts": "WriteConflictsOp",
	"total_keys_scanned":        "TotalKeysScanned",
	"total_docs_scanned":        "TotalObjectsScanned",
}

var DefaultAssertsStats = map[string]string{
	"assert_regular":   "Regular",
	"assert_warning":   "Warning",
	"assert_msg":       "Msg",
	"assert_user":      "User",
	"assert_rollovers": "Rollovers",
}

var DefaultCommandsStats = map[string]string{
	"aggregate_command_total":        "AggregateCommandTotal",
	"aggregate_command_failed":       "AggregateCommandFailed",
	"count_command_total":            "CountCommandTotal",
	"count_command_failed":           "CountCommandFailed",
	"delete_command_total":           "DeleteCommandTotal",
	"delete_command_failed":          "DeleteCommandFailed",
	"distinct_command_total":         "DistinctCommandTotal",
	"distinct_command_failed":        "DistinctCommandFailed",
	"find_command_total":             "FindCommandTotal",
	"find_command_failed":            "FindCommandFailed",
	"find_and_modify_command_total":  "FindAndModifyCommandTotal",
	"find_and_modify_command_failed": "FindAndModifyCommandFailed",
	"get_more_command_total":         "GetMoreCommandTotal",
	"get_more_command_failed":        "GetMoreCommandFailed",
	"insert_command_total":           "InsertCommandTotal",
	"insert_command_failed":          "InsertCommandFailed",
	"update_command_total":           "UpdateCommandTotal",
	"update_command_failed":          "UpdateCommandFailed",
}

var DefaultLatencyStats = map[string]string{
	"latency_writes_count":   "WriteOpsCnt",
	"latency_writes":         "WriteLatency",
	"latency_reads_count":    "ReadOpsCnt",
	"latency_reads":          "ReadLatency",
	"latency_commands_count": "CommandOpsCnt",
	"latency_commands":       "CommandLatency",
}

var DefaultReplStats = map[string]string{
	"repl_inserts":                             "InsertRCnt",
	"repl_inserts_per_sec":                     "InsertR",
	"repl_queries":                             "QueryRCnt",
	"repl_queries_per_sec":                     "QueryR",
	"repl_updates":                             "UpdateRCnt",
	"repl_updates_per_sec":                     "UpdateR",
	"repl_deletes":                             "DeleteRCnt",
	"repl_deletes_per_sec":                     "DeleteR",
	"repl_getmores":                            "GetMoreRCnt",
	"repl_getmores_per_sec":                    "GetMoreR",
	"repl_commands":                            "CommandRCnt",
	"repl_commands_per_sec":                    "CommandR",
	"member_status":                            "NodeType",
	"state":                                    "NodeState",
	"repl_state":                               "NodeStateInt",
	"repl_lag":                                 "ReplLag",
	"repl_network_bytes":                       "ReplNetworkBytes",
	"repl_network_getmores_num":                "ReplNetworkGetmoresNum",
	"repl_network_getmores_total_millis":       "ReplNetworkGetmoresTotalMillis",
	"repl_network_ops":                         "ReplNetworkOps",
	"repl_buffer_count":                        "ReplBufferCount",
	"repl_buffer_size_bytes":                   "ReplBufferSizeBytes",
	"repl_apply_batches_num":                   "ReplApplyBatchesNum",
	"repl_apply_batches_total_millis":          "ReplApplyBatchesTotalMillis",
	"repl_apply_ops":                           "ReplApplyOps",
	"repl_executor_pool_in_progress_count":     "ReplExecutorPoolInProgressCount",
	"repl_executor_queues_network_in_progress": "ReplExecutorQueuesNetworkInProgress",
	"repl_executor_queues_sleepers":            "ReplExecutorQueuesSleepers",
	"repl_executor_unsignaled_events":          "ReplExecutorUnsignaledEvents",
}

var DefaultClusterStats = map[string]string{
	"jumbo_chunks": "JumboChunksCount",
}

var DefaultShardStats = map[string]string{
	"total_in_use":     "TotalInUse",
	"total_available":  "TotalAvailable",
	"total_created":    "TotalCreated",
	"total_refreshing": "TotalRefreshing",
}

var ShardHostStats = map[string]string{
	"in_use":     "InUse",
	"available":  "Available",
	"created":    "Created",
	"refreshing": "Refreshing",
}

var MmapStats = map[string]string{
	"mapped_megabytes":     "Mapped",
	"non-mapped_megabytes": "NonMapped",
	"page_faults":          "FaultsCnt",
	"page_faults_per_sec":  "Faults",
}

var WiredTigerStats = map[string]string{
	"percent_cache_dirty": "CacheDirtyPercent",
	"percent_cache_used":  "CacheUsedPercent",
}

var WiredTigerExtStats = map[string]string{
	"wtcache_tracked_dirty_bytes":          "TrackedDirtyBytes",
	"wtcache_current_bytes":                "CurrentCachedBytes",
	"wtcache_max_bytes_configured":         "MaxBytesConfigured",
	"wtcache_app_threads_page_read_count":  "AppThreadsPageReadCount",
	"wtcache_app_threads_page_read_time":   "AppThreadsPageReadTime",
	"wtcache_app_threads_page_write_count": "AppThreadsPageWriteCount",
	"wtcache_bytes_written_from":           "BytesWrittenFrom",
	"wtcache_bytes_read_into":              "BytesReadInto",
	"wtcache_pages_evicted_by_app_thread":  "PagesEvictedByAppThread",
	"wtcache_pages_queued_for_eviction":    "PagesQueuedForEviction",
	"wtcache_pages_read_into":              "PagesReadIntoCache",
	"wtcache_pages_written_from":           "PagesWrittenFromCache",
	"wtcache_pages_requested_from":         "PagesRequestedFromCache",
	"wtcache_server_evicting_pages":        "ServerEvictingPages",
	"wtcache_worker_thread_evictingpages":  "WorkerThreadEvictingPages",
	"wtcache_internal_pages_evicted":       "InternalPagesEvicted",
	"wtcache_modified_pages_evicted":       "ModifiedPagesEvicted",
	"wtcache_unmodified_pages_evicted":     "UnmodifiedPagesEvicted",
}

var DefaultTCMallocStats = map[string]string{
	"tcmalloc_current_allocated_bytes":          "TCMallocCurrentAllocatedBytes",
	"tcmalloc_heap_size":                        "TCMallocHeapSize",
	"tcmalloc_central_cache_free_bytes":         "TCMallocCentralCacheFreeBytes",
	"tcmalloc_current_total_thread_cache_bytes": "TCMallocCurrentTotalThreadCacheBytes",
	"tcmalloc_max_total_thread_cache_bytes":     "TCMallocMaxTotalThreadCacheBytes",
	"tcmalloc_total_free_bytes":                 "TCMallocTotalFreeBytes",
	"tcmalloc_transfer_cache_free_bytes":        "TCMallocTransferCacheFreeBytes",
	"tcmalloc_thread_cache_free_bytes":          "TCMallocThreadCacheFreeBytes",
	"tcmalloc_spinlock_total_delay_ns":          "TCMallocSpinLockTotalDelayNanos",
	"tcmalloc_pageheap_free_bytes":              "TCMallocPageheapFreeBytes",
	"tcmalloc_pageheap_unmapped_bytes":          "TCMallocPageheapUnmappedBytes",
	"tcmalloc_pageheap_committed_bytes":         "TCMallocPageheapComittedBytes",
	"tcmalloc_pageheap_scavenge_count":          "TCMallocPageheapScavengeCount",
	"tcmalloc_pageheap_commit_count":            "TCMallocPageheapCommitCount",
	"tcmalloc_pageheap_total_commit_bytes":      "TCMallocPageheapTotalCommitBytes",
	"tcmalloc_pageheap_decommit_count":          "TCMallocPageheapDecommitCount",
	"tcmalloc_pageheap_total_decommit_bytes":    "TCMallocPageheapTotalDecommitBytes",
	"tcmalloc_pageheap_reserve_count":           "TCMallocPageheapReserveCount",
	"tcmalloc_pageheap_total_reserve_bytes":     "TCMallocPageheapTotalReserveBytes",
}

var DefaultStorageStats = map[string]string{
	"storage_freelist_search_bucket_exhausted": "StorageFreelistSearchBucketExhausted",
	"storage_freelist_search_requests":         "StorageFreelistSearchRequests",
	"storage_freelist_search_scanned":          "StorageFreelistSearchScanned",
}

var DbDataStats = map[string]string{
	"collections":  "Collections",
	"objects":      "Objects",
	"avg_obj_size": "AvgObjSize",
	"data_size":    "DataSize",
	"storage_size": "StorageSize",
	"num_extents":  "NumExtents",
	"indexes":      "Indexes",
	"index_size":   "IndexSize",
	"ok":           "Ok",
}

var ColDataStats = map[string]string{
	"count":            "Count",
	"size":             "Size",
	"avg_obj_size":     "AvgObjSize",
	"storage_size":     "StorageSize",
	"total_index_size": "TotalIndexSize",
	"ok":               "Ok",
}

func (d *MongodbData) AddDbStats() {
	for _, dbstat := range d.StatLine.DbStatsLines {
		dbStatLine := reflect.ValueOf(&dbstat).Elem()
		newDbData := &DbData{
			Name:   dbstat.Name,
			Fields: make(map[string]interface{}),
		}
		newDbData.Fields["type"] = "db_stat"
		for key, value := range DbDataStats {
			val := dbStatLine.FieldByName(value).Interface()
			newDbData.Fields[key] = val
		}
		d.DbData = append(d.DbData, *newDbData)
	}
}

func (d *MongodbData) AddColStats() {
	for _, colstat := range d.StatLine.ColStatsLines {
		colStatLine := reflect.ValueOf(&colstat).Elem()
		newColData := &ColData{
			Name:   colstat.Name,
			DbName: colstat.DbName,
			Fields: make(map[string]interface{}),
		}
		newColData.Fields["type"] = "col_stat"
		for key, value := range ColDataStats {
			val := colStatLine.FieldByName(value).Interface()
			newColData.Fields[key] = val
		}
		d.ColData = append(d.ColData, *newColData)
	}
}

func (d *MongodbData) AddShardHostStats() {
	for host, hostStat := range d.StatLine.ShardHostStatsLines {
		hostStatLine := reflect.ValueOf(&hostStat).Elem()
		newDbData := &DbData{
			Name:   host,
			Fields: make(map[string]interface{}),
		}
		newDbData.Fields["type"] = "shard_host_stat"
		for k, v := range ShardHostStats {
			val := hostStatLine.FieldByName(v).Interface()
			newDbData.Fields[k] = val
		}
		d.ShardHostData = append(d.ShardHostData, *newDbData)
	}
}

func (d *MongodbData) AddDefaultStats() {
	statLine := reflect.ValueOf(d.StatLine).Elem()
	d.addStat(statLine, DefaultStats)
	if d.StatLine.NodeType != "" {
		d.addStat(statLine, DefaultReplStats)
		d.Tags["node_type"] = d.StatLine.NodeType
	}

	if d.StatLine.ReadLatency > 0 {
		d.addStat(statLine, DefaultLatencyStats)
	}

	if d.StatLine.ReplSetName != "" {
		d.Tags["rs_name"] = d.StatLine.ReplSetName
	}

	if d.StatLine.OplogStats != nil {
		d.add("repl_oplog_window_sec", d.StatLine.OplogStats.TimeDiff)
	}

	if d.StatLine.Version != "" {
		d.add("version", d.StatLine.Version)
	}

	d.addStat(statLine, DefaultAssertsStats)
	d.addStat(statLine, DefaultClusterStats)
	d.addStat(statLine, DefaultCommandsStats)
	d.addStat(statLine, DefaultShardStats)
	d.addStat(statLine, DefaultStorageStats)
	d.addStat(statLine, DefaultTCMallocStats)

	if d.StatLine.StorageEngine == "mmapv1" || d.StatLine.StorageEngine == "rocksdb" {
		d.addStat(statLine, MmapStats)
	} else if d.StatLine.StorageEngine == "wiredTiger" {
		for key, value := range WiredTigerStats {
			val := statLine.FieldByName(value).Interface()
			percentVal := fmt.Sprintf("%.1f", val.(float64)*100)
			floatVal, _ := strconv.ParseFloat(percentVal, 64)
			d.add(key, floatVal)
		}
		d.addStat(statLine, WiredTigerExtStats)
		d.add("page_faults", d.StatLine.FaultsCnt)
	}
}

func (d *MongodbData) addStat(statLine reflect.Value, stats map[string]string) {
	for key, value := range stats {
		val := statLine.FieldByName(value).Interface()
		d.add(key, val)
	}
}

func (d *MongodbData) add(key string, val interface{}) {
	d.Fields[key] = val
}

func (d *MongodbData) flush(acc telegraf.Accumulator) {
	acc.AddFields(
		"mongodb",
		d.Fields,
		d.Tags,
		d.StatLine.Time,
	)
	d.Fields = make(map[string]interface{})

	for _, db := range d.DbData {
		d.Tags["db_name"] = db.Name
		acc.AddFields(
			"mongodb_db_stats",
			db.Fields,
			d.Tags,
			d.StatLine.Time,
		)
		db.Fields = make(map[string]interface{})
	}
	for _, col := range d.ColData {
		d.Tags["collection"] = col.Name
		d.Tags["db_name"] = col.DbName
		acc.AddFields(
			"mongodb_col_stats",
			col.Fields,
			d.Tags,
			d.StatLine.Time,
		)
		col.Fields = make(map[string]interface{})
	}
	for _, host := range d.ShardHostData {
		d.Tags["hostname"] = host.Name
		acc.AddFields(
			"mongodb_shard_stats",
			host.Fields,
			d.Tags,
			d.StatLine.Time,
		)
		host.Fields = make(map[string]interface{})
	}
}
