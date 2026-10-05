package repo

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/attachment"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entity"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entityfield"
	"gocloud.dev/blob"
	"strings"
	"testing"
)

func TestLocationNames(t *testing.T) {
	o := LocationOperation{Mode: "number", Count: 12, Start: 1, Pattern: "Fach * Sortierkasten blau"}
	names, err := o.GenerateNames()
	require.NoError(t, err)
	require.Len(t, names, 12)
	require.Equal(t, "Fach 12 Sortierkasten blau", names[11])
	o.Start = 7
	o.Digits = 3
	names, err = o.GenerateNames()
	require.NoError(t, err)
	require.Equal(t, "Fach 007 Sortierkasten blau", names[0])
	o = LocationOperation{Mode: "grid", Rows: 3, Columns: 4, RowStart: "A", Start: 1, Pattern: "Fach {row}{col}"}
	names, err = o.GenerateNames()
	require.NoError(t, err)
	require.Len(t, names, 12)
	require.Equal(t, "Fach A1", names[0])
	require.Equal(t, "Fach C4", names[11])
	o.RowStart = "D"
	o.Start = 8
	names, err = o.GenerateNames()
	require.NoError(t, err)
	require.Equal(t, "Fach D8", names[0])
	for _, bad := range []LocationOperation{
		{Mode: "number", Count: 0, Start: 1, Pattern: "Fach *"}, {Mode: "number", Count: 201, Start: 1, Pattern: "Fach *"}, {Mode: "number", Count: 1, Start: 1, Pattern: "no placeholder"}, {Mode: "number", Count: 1, Start: 1, Pattern: "**"}, {Mode: "grid", Rows: 3, Columns: 4, RowStart: "Z", Pattern: "{row}{col}"}, {Mode: "grid", Rows: 20, Columns: 20, RowStart: "A", Pattern: "{row}{col}"}, {Mode: "grid", Rows: 1, Columns: 1, RowStart: "A", Pattern: "{row}{col}{unknown}"},
	} {
		_, err = bad.GenerateNames()
		require.Error(t, err)
	}
}
func TestLocationOperations(t *testing.T) {
	ctx := context.Background()
	_, err := tClient.Sql().Exec(`CREATE TABLE IF NOT EXISTS location_operations (group_id uuid NOT NULL, request_id uuid NOT NULL, fingerprint text NOT NULL, result text NOT NULL, PRIMARY KEY(group_id, request_id))`)
	require.NoError(t, err)
	locType := useContainerEntityType(t)
	itemType := useItemEntityType(t)
	create := func(name string, parent uuid.UUID, location bool) uuid.UUID {
		typ := itemType.ID
		if location {
			typ = locType.ID
		}
		e, err := tRepos.Entities.Create(ctx, tGroup.ID, EntityCreate{Name: name, ParentID: parent, EntityTypeID: typ, Quantity: 7})
		require.NoError(t, err)
		return e.ID
	}
	root := create("Operation source", uuid.Nil, true)
	child := create("Child", root, true)
	grand := create("Grandchild", child, true)
	_ = create("Deep", grand, true)
	item := create("Tool", child, false)
	_ = create("Tool part", item, false)
	_, err = tClient.Entity.UpdateOneID(item).SetSerialNumber("private").SetPurchaseFrom("shop").SetLifetimeWarranty(true).SetNotes("independent notes").Save(ctx)
	require.NoError(t, err)
	_, err = tClient.EntityField.Create().SetEntityID(child).SetName("account").SetType(entityfield.TypeText).SetTextValue("personal account").Save(ctx)
	require.NoError(t, err)
	o := LocationOperation{Action: "generate", RequestID: uuid.New(), Mode: "number", Count: 12, Start: 1, Pattern: "Fach *", Depth: -1}
	first, err := tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.NoError(t, err)
	require.Equal(t, 12, first.Locations)
	replay, err := tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, first.RootID, replay.RootID)
	direct, err := tClient.Entity.Query().Where(entity.HasParentWith(entity.ID(root))).All(ctx)
	require.NoError(t, err)
	require.Len(t, direct, 13)
	o.Count = 2
	_, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.ErrorContains(t, err, "request_reused")
	o.RequestID = uuid.New()
	o.Preview = true
	preview, err := tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.NoError(t, err)
	require.Len(t, preview.Conflicts, 2)
	o.Preview = false
	_, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.ErrorContains(t, err, "conflicts")
	o.AllowConflicts = true
	_, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.NoError(t, err)
	o = LocationOperation{Action: "generate", RequestID: uuid.New(), Mode: "grid", Rows: 3, Columns: 4, RowStart: "A", Start: 1, Pattern: "Fach {row}{col}"}
	_, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, grand, o)
	require.NoError(t, err)
	for _, depth := range []int{0, 1, 2, -1} {
		t.Run(fmt.Sprintf("depth%d", depth), func(t *testing.T) {
			o := LocationOperation{Action: "copy", RequestID: uuid.New(), Name: fmt.Sprintf("copy %d", depth), Depth: depth, Items: true, Preview: true, AllowConflicts: true}
			preview, err := tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
			require.NoError(t, err)
			if depth == 0 {
				require.Equal(t, 1, preview.Locations)
				require.Zero(t, preview.Items)
			} else {
				require.Equal(t, 2, preview.Items)
			}
			o.Preview = false
			out, err := tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
			require.NoError(t, err)
			require.NotEqual(t, root, out.RootID)
			copied, err := tClient.Entity.Query().Where(entity.ID(out.RootID)).WithChildren().Only(ctx)
			require.NoError(t, err)
			require.Greater(t, copied.AssetID, int64(0))
			if depth != 0 {
				copiedChild, err := tClient.Entity.Query().Where(entity.HasParentWith(entity.ID(out.RootID)), entity.NameEQ("Child")).Only(ctx)
				require.NoError(t, err)
				copiedItem, err := tClient.Entity.Query().Where(entity.HasParentWith(entity.ID(copiedChild.ID)), entity.NameEQ("Tool")).Only(ctx)
				require.NoError(t, err)
				require.NotEqual(t, item, copiedItem.ID)
				require.Equal(t, float64(7), copiedItem.Quantity)
				require.Empty(t, copiedItem.SerialNumber)
				require.Empty(t, copiedItem.PurchaseFrom)
				require.False(t, copiedItem.LifetimeWarranty)
				require.Equal(t, "independent notes", copiedItem.Notes)
				fields, err := copiedChild.QueryFields().All(ctx)
				require.NoError(t, err)
				require.Len(t, fields, 1)
				require.Equal(t, "personal account", fields[0].TextValue)
			}
		})
	}
	o = LocationOperation{Action: "copy", RequestID: uuid.New(), Name: "No items", Depth: -1}
	out, err := tRepos.Entities.OperateLocation(ctx, tGroup.ID, child, o)
	require.NoError(t, err)
	require.Zero(t, out.Items)
	o = LocationOperation{Action: "copy", RequestID: uuid.New(), Name: "Individual details", Depth: 0, Items: true, SerialNumbers: true, PurchaseWarranty: true}
	out, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, child, o)
	require.NoError(t, err)
	copiedItem, err := tClient.Entity.Query().Where(entity.HasParentWith(entity.ID(out.RootID)), entity.NameEQ("Tool")).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "private", copiedItem.SerialNumber)
	require.Equal(t, "shop", copiedItem.PurchaseFrom)
	require.True(t, copiedItem.LifetimeWarranty)
	// Same-group duplicate names are explicit and never replace a record.
	o = LocationOperation{Action: "copy", RequestID: uuid.New(), Name: "Operation source", Depth: 0, Preview: true}
	out, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.NoError(t, err)
	require.NotEmpty(t, out.Conflicts)
	// Copying into the source's descendants uses the fixed snapshot exactly once.
	o = LocationOperation{Action: "copy", RequestID: uuid.New(), Name: "Snapshot", ParentID: grand, Depth: -1}
	out, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, child, o)
	require.NoError(t, err)
	require.Equal(t, 15, out.Locations)
	destination := create("Destination", uuid.Nil, true)
	o = LocationOperation{Action: "move", RequestID: uuid.New(), Name: "Moved", ParentID: destination, Depth: -1}
	out, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, child, o)
	require.NoError(t, err)
	require.Equal(t, child, out.RootID)
	unchanged, err := tClient.Entity.Query().Where(entity.ID(item)).WithParent().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, child, unchanged.Edges.Parent.ID)
	out, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, child, o)
	require.NoError(t, err)
	require.True(t, out.Replayed)
	o.RequestID = uuid.New()
	o.ParentID = grand
	_, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, child, o)
	require.ErrorContains(t, err, "cycle")
	_, err = tRepos.Entities.OperateLocation(ctx, uuid.New(), child, o)
	require.ErrorContains(t, err, "location_not_found")
	other, err := tRepos.Groups.GroupCreate(ctx, "other operations", uuid.Nil)
	require.NoError(t, err)
	foreign, err := tRepos.Entities.Create(ctx, other.ID, EntityCreate{Name: "foreign"})
	require.NoError(t, err)
	o.ParentID = foreign.ID
	_, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, child, o)
	require.ErrorContains(t, err, "location_not_found")
	// Independent files survive deletion, including thumbnail relationships.
	bucket, err := blob.OpenBucket(ctx, tRepos.Attachments.GetConnString())
	require.NoError(t, err)
	defer bucket.Close()
	path := tRepos.Attachments.path(tGroup.ID, "original-"+uuid.NewString())
	require.NoError(t, bucket.WriteAll(ctx, tRepos.Attachments.fullPath(path), []byte("photo test"), nil))
	photo, err := tClient.Attachment.Create().SetEntityID(root).SetPath(path).SetType(attachment.TypePhoto).SetPrimary(true).SetMimeType("image/png").Save(ctx)
	require.NoError(t, err)
	o = LocationOperation{Action: "copy", RequestID: uuid.New(), Name: "Photo copy", Depth: 0, Photos: true}
	out, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.NoError(t, err)
	copiedPhoto, err := tClient.Attachment.Query().Where(attachment.HasEntityWith(entity.ID(out.RootID))).Only(ctx)
	require.NoError(t, err)
	require.NotEqual(t, path, copiedPhoto.Path)
	require.NoError(t, tRepos.Attachments.Delete(ctx, tGroup.ID, photo.ID))
	data, err := bucket.ReadAll(ctx, tRepos.Attachments.fullPath(copiedPhoto.Path))
	require.NoError(t, err)
	require.Equal(t, "photo test", string(data))
	// A missing source file aborts every database insert.
	_, err = tClient.Attachment.Create().SetEntityID(root).SetPath("nonexistent/" + uuid.NewString()).SetType(attachment.TypePhoto).Save(ctx)
	require.NoError(t, err)
	o.RequestID = uuid.New()
	o.Name = "Must rollback"
	_, err = tRepos.Entities.OperateLocation(ctx, tGroup.ID, root, o)
	require.Error(t, err)
	exists, err := tClient.Entity.Query().Where(entity.NameEQ("Must rollback")).Exist(ctx)
	require.NoError(t, err)
	require.False(t, exists)
	// Requests and names remain literal; no executable templating.
	_, err = (LocationOperation{Mode: "number", Count: 1, Start: 1, Pattern: strings.Repeat("x", 256) + "*"}).GenerateNames()
	require.Error(t, err)
}
