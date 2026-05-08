package v2

import (
	"context"

	"github.com/tencent-connect/botgo/dto"
)

// GroupUploadPrepare 群 - 创建分片上传任务
func (o *openAPIv2) GroupUploadPrepare(ctx context.Context, groupID string, req *dto.UploadPrepareRequest) (*dto.UploadPrepareResponse, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UploadPrepareResponse{}).
		SetPathParam("group_id", groupID).
		SetBody(req).
		Post(o.getURL(groupUploadPrepareURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UploadPrepareResponse), nil
}

// GroupUploadPartFinish 群 - 上报分片完成
func (o *openAPIv2) GroupUploadPartFinish(ctx context.Context, groupID string, req *dto.UploadPartFinishRequest) (*dto.UploadPartFinishResponse, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UploadPartFinishResponse{}).
		SetPathParam("group_id", groupID).
		SetBody(req).
		Post(o.getURL(groupUploadPartFinishURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UploadPartFinishResponse), nil
}

// GroupPostFile 群 - 合并完成分片上传（带 upload_id）
func (o *openAPIv2) GroupPostFile(ctx context.Context, groupID string, req *dto.PostFileRequest) (*dto.MediaResponse, error) {
	resp, err := o.request(ctx).
		SetResult(dto.MediaResponse{}).
		SetPathParam("group_id", groupID).
		SetBody(req).
		Post(o.getURL(groupRichMediaURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.MediaResponse), nil
}

// C2CUploadPrepare C2C - 创建分片上传任务
func (o *openAPIv2) C2CUploadPrepare(ctx context.Context, userID string, req *dto.UploadPrepareRequest) (*dto.UploadPrepareResponse, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UploadPrepareResponse{}).
		SetPathParam("user_id", userID).
		SetBody(req).
		Post(o.getURL(c2cUploadPrepareURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UploadPrepareResponse), nil
}

// C2CUploadPartFinish C2C - 上报分片完成
func (o *openAPIv2) C2CUploadPartFinish(ctx context.Context, userID string, req *dto.UploadPartFinishRequest) (*dto.UploadPartFinishResponse, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UploadPartFinishResponse{}).
		SetPathParam("user_id", userID).
		SetBody(req).
		Post(o.getURL(c2cUploadPartFinishURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UploadPartFinishResponse), nil
}

// C2CPostFile C2C - 合并完成分片上传（带 upload_id）
func (o *openAPIv2) C2CPostFile(ctx context.Context, userID string, req *dto.PostFileRequest) (*dto.MediaResponse, error) {
	resp, err := o.request(ctx).
		SetResult(dto.MediaResponse{}).
		SetPathParam("user_id", userID).
		SetBody(req).
		Post(o.getURL(c2cRichMediaURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.MediaResponse), nil
}
