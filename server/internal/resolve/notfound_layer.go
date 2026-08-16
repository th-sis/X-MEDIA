package resolve

import (
	"context"

	"xmedia/internal/domain"
	"xmedia/internal/websocket"
)

func (s *Service) notFound(t *domain.ResolveTask) {
	t.Status = domain.ResolveFailed
	t.Stage = domain.StageNotFound
	t.StageDetail = "暂无可用资源"
	t.ErrorMsg = "暂无可用资源"
	_ = s.tasks.Update(context.Background(), t)
	if s.subs != nil {
		_, _ = s.subs.Add(context.Background(), &domain.Subscription{
			ExternalID:     t.ExternalID,
			ExternalSource: t.ExternalSource,
			MediaType:      t.MediaType,
			Title:          t.Title,
			Year:           t.Year,
			Status:         domain.SubWatching,
			MaxSearches:    12,
		})
	}
	if s.hub != nil {
		s.hub.Broadcast(websocket.TypeResolveFailed, websocket.ResolveFailedPayload{
			TaskID:     t.ID,
			Reason:     "暂无可用资源",
			Suggestion: "已自动创建订阅，系统将每周自动搜寻",
			Stage:      string(domain.StageNotFound),
		})
	}
}
