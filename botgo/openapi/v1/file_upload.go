package v1

import (
	"context"
	"fmt"

	"github.com/tencent-connect/botgo/dto"
)

var errNotSupported = fmt.Errorf("chunked file upload is not supported in openapi v1, use v2")

func (o *openAPI) GroupUploadPrepare(ctx context.Context, groupID string, req *dto.UploadPrepareRequest) (*dto.UploadPrepareResponse, error) {
	return nil, errNotSupported
}

func (o *openAPI) GroupUploadPartFinish(ctx context.Context, groupID string, req *dto.UploadPartFinishRequest) (*dto.UploadPartFinishResponse, error) {
	return nil, errNotSupported
}

func (o *openAPI) GroupPostFile(ctx context.Context, groupID string, req *dto.PostFileRequest) (*dto.MediaResponse, error) {
	return nil, errNotSupported
}

func (o *openAPI) C2CUploadPrepare(ctx context.Context, userID string, req *dto.UploadPrepareRequest) (*dto.UploadPrepareResponse, error) {
	return nil, errNotSupported
}

func (o *openAPI) C2CUploadPartFinish(ctx context.Context, userID string, req *dto.UploadPartFinishRequest) (*dto.UploadPartFinishResponse, error) {
	return nil, errNotSupported
}

func (o *openAPI) C2CPostFile(ctx context.Context, userID string, req *dto.PostFileRequest) (*dto.MediaResponse, error) {
	return nil, errNotSupported
}
