package v1

import (
	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
	"github.com/sysadminsmedia/homebox/backend/internal/web/adapters"
	"net/http"
)

func (ctrl *V1Controller) HandleLocationOperation() errchain.HandlerFunc {
	handler := adapters.ActionID("id", func(r *http.Request, id uuid.UUID, data repo.LocationOperation) (repo.LocationOperationResult, error) {
		ctx := services.NewContext(r.Context())
		result, err := ctrl.repo.Entities.OperateLocation(ctx, ctx.GID, id, data)
		if err != nil {
			return result, validate.NewRequestError(err, http.StatusBadRequest)
		}
		return result, nil
	}, http.StatusOK)
	return func(w http.ResponseWriter, r *http.Request) error {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		return handler(w, r)
	}
}
