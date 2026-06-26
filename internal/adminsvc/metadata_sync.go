package adminsvc

import "context"

const metadataSyncChannel = "ddag:metadata:sync"

func (s *service) publishMetadataSync(ctx context.Context, reason string) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Publish(ctx, metadataSyncChannel, reason).Err(); err != nil {
		s.log.Warn("metadata_sync_publish_failed", "reason", reason, "error", err.Error())
	}
}
