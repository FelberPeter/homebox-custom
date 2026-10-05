package repo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	entsql "entgo.io/ent/dialect/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entity"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"gocloud.dev/blob"
)

const LocationOperationLimit = 200

var locationOperationMutex sync.Mutex

type LocationOperation struct {
	Action           string    `json:"action"`
	RequestID        uuid.UUID `json:"requestId"`
	Preview          bool      `json:"preview"`
	AllowConflicts   bool      `json:"allowConflicts"`
	Name             string    `json:"name"`
	ParentID         uuid.UUID `json:"parentId"`
	Depth            int       `json:"depth"`
	Items            bool      `json:"items"`
	Photos           bool      `json:"photos"`
	Attachments      bool      `json:"attachments"`
	SerialNumbers    bool      `json:"serialNumbers"`
	PurchaseWarranty bool      `json:"purchaseWarranty"`
	Mode             string    `json:"mode"`
	Count            int       `json:"count"`
	Start            int       `json:"start"`
	Digits           int       `json:"digits"`
	Rows             int       `json:"rows"`
	Columns          int       `json:"columns"`
	RowStart         string    `json:"rowStart"`
	Pattern          string    `json:"pattern"`
	Description      string    `json:"description"`
}
type LocationOperationNode struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Depth    int       `json:"depth"`
	Location bool      `json:"location"`
}
type LocationOperationResult struct {
	Nodes         []LocationOperationNode `json:"nodes"`
	Conflicts     []string                `json:"conflicts"`
	Locations     int                     `json:"locations"`
	Items         int                     `json:"items"`
	Excluded      int                     `json:"excluded"`
	ExcludedNodes []LocationOperationNode `json:"excludedNodes"`
	RootID        uuid.UUID               `json:"rootId"`
	Replayed      bool                    `json:"replayed"`
}

func (o LocationOperation) GenerateNames() ([]string, error) {
	if o.Start < 0 || o.Start > 1000000 || o.Digits < 0 || o.Digits > 8 {
		return nil, fmt.Errorf("invalid_numbers")
	}
	var names []string
	switch o.Mode {
	case "number":
		if o.Count < 1 || o.Count > LocationOperationLimit || strings.Count(o.Pattern, "*") != 1 || strings.ContainsAny(o.Pattern, "{}") {
			return nil, fmt.Errorf("invalid_pattern")
		}
		for i := 0; i < o.Count; i++ {
			names = append(names, strings.ReplaceAll(o.Pattern, "*", fmt.Sprintf("%0*d", o.Digits, o.Start+i)))
		}
	case "grid":
		if o.Rows < 1 || o.Columns < 1 || o.Rows > 26 || o.Columns > LocationOperationLimit || o.Rows*o.Columns > LocationOperationLimit || len(o.RowStart) != 1 || o.RowStart[0] < 'A' || o.RowStart[0] > 'Z' || int(o.RowStart[0])+o.Rows-1 > int('Z') {
			return nil, fmt.Errorf("invalid_numbers")
		}
		if strings.Count(o.Pattern, "{row}") != 1 || strings.Count(o.Pattern, "{col}") != 1 || strings.ContainsAny(strings.NewReplacer("{row}", "", "{col}", "").Replace(o.Pattern), "{}*") {
			return nil, fmt.Errorf("invalid_pattern")
		}
		for row := 0; row < o.Rows; row++ {
			for col := 0; col < o.Columns; col++ {
				names = append(names, strings.NewReplacer("{row}", string(rune(o.RowStart[0])+rune(row)), "{col}", strconv.Itoa(o.Start+col)).Replace(o.Pattern))
			}
		}
	default:
		return nil, fmt.Errorf("invalid_pattern")
	}
	for _, n := range names {
		if strings.TrimSpace(n) == "" || utf8.RuneCountInString(n) > 255 {
			return nil, fmt.Errorf("invalid_name")
		}
	}
	return names, nil
}

// OperateLocation snapshots the tenant tree before creating any copies. Database
// changes commit together. Files use fresh keys; interrupted staging can leave
// unreferenced files, but never a partial tree or a shared original file.
func (r *EntityRepository) OperateLocation(ctx context.Context, gid, sourceID uuid.UUID, o LocationOperation) (result LocationOperationResult, err error) {
	locationOperationMutex.Lock()
	defer locationOperationMutex.Unlock()
	result.Nodes = []LocationOperationNode{}
	result.ExcludedNodes = []LocationOperationNode{}
	result.Conflicts = []string{}
	if o.RequestID == uuid.Nil || (o.Action != "generate" && o.Action != "copy" && o.Action != "move") || o.Depth < -1 || o.Depth > 100 {
		return result, fmt.Errorf("invalid_request")
	}
	identity := o
	identity.Preview = false
	identity.AllowConflicts = false
	raw, _ := json.Marshal(identity)
	raw = append(raw, sourceID[:]...)
	hash := sha256.Sum256(raw)
	receipt := fmt.Sprintf("custom:%s:%x", o.RequestID, hash[:12])
	rootID := uuid.NewSHA1(gid, []byte(o.RequestID.String()))
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	all, err := tx.Entity.Query().Where(entity.HasGroupWith(group.ID(gid))).WithParent().WithEntityType().WithTag().WithFields().WithAttachments(func(q *ent.AttachmentQuery) { q.WithThumbnail() }).All(ctx)
	if err != nil {
		return result, err
	}
	byID := map[uuid.UUID]*ent.Entity{}
	children := map[uuid.UUID][]*ent.Entity{}
	for _, e := range all {
		byID[e.ID] = e
		if e.Edges.Parent != nil {
			children[e.Edges.Parent.ID] = append(children[e.Edges.Parent.ID], e)
		}
	}
	driver := tx.SQLDriver()
	builder := entsql.Dialect(driver.Dialect())
	query, args := builder.Select("fingerprint", "result").From(entsql.Table("location_operations")).Where(entsql.And(entsql.EQ("group_id", gid), entsql.EQ("request_id", o.RequestID))).Query()
	rows := &entsql.Rows{}
	if err = driver.Query(ctx, query, args, rows); err != nil {
		return result, err
	}
	if rows.Next() {
		var fingerprint, stored string
		if err = rows.Scan(&fingerprint, &stored); err != nil {
			_ = rows.Close()
			return result, err
		}
		_ = rows.Close()
		if fingerprint != receipt {
			return result, fmt.Errorf("request_reused")
		}
		err = json.Unmarshal([]byte(stored), &result)
		result.Replayed = true
		return result, err
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return result, err
	}
	_ = rows.Close()
	commit := func() error {
		stored, e := json.Marshal(result)
		if e != nil {
			return e
		}
		query, args := builder.Insert("location_operations").Columns("group_id", "request_id", "fingerprint", "result").Values(gid, o.RequestID, receipt, string(stored)).Query()
		if e = driver.Exec(ctx, query, args, nil); e != nil {
			return e
		}
		return tx.Commit()
	}
	source := byID[sourceID]
	if source == nil || source.Edges.EntityType == nil || !source.Edges.EntityType.IsLocation {
		return result, fmt.Errorf("location_not_found")
	}
	if o.ParentID != uuid.Nil {
		p := byID[o.ParentID]
		if p == nil || !p.Edges.EntityType.IsLocation {
			return result, fmt.Errorf("location_not_found")
		}
	}
	if (o.Action == "copy" || o.Action == "move") && (strings.TrimSpace(o.Name) == "" || utf8.RuneCountInString(o.Name) > 255) {
		return result, fmt.Errorf("invalid_name")
	}
	if o.Action == "move" {
		o.Depth = -1
		o.Items = true
	}
	var selected []*ent.Entity
	var visit func(*ent.Entity, int, bool) error
	seen := map[uuid.UUID]bool{}
	visit = func(e *ent.Entity, depth int, include bool) error {
		if seen[e.ID] {
			return fmt.Errorf("cycle")
		}
		seen[e.ID] = true
		loc := e.Edges.EntityType.IsLocation
		if include {
			if o.Action != "move" && len(selected) >= LocationOperationLimit {
				return fmt.Errorf("limit")
			}
			selected = append(selected, e)
			name := e.Name
			if e.ID == sourceID {
				name = o.Name
			}
			result.Nodes = append(result.Nodes, LocationOperationNode{e.ID, name, depth, loc})
			if loc {
				result.Locations++
			} else {
				result.Items++
			}
		} else {
			result.Excluded++
			result.ExcludedNodes = append(result.ExcludedNodes, LocationOperationNode{e.ID, e.Name, depth, loc})
		}
		for _, c := range children[e.ID] {
			childLoc := c.Edges.EntityType.IsLocation
			nextDepth := depth
			if childLoc {
				nextDepth++
			}
			take := include && ((!childLoc && o.Items) || (childLoc && (o.Depth == -1 || nextDepth <= o.Depth)))
			if err := visit(c, nextDepth, take); err != nil {
				return err
			}
		}
		return nil
	}
	var names []string
	if o.Action == "generate" {
		names, err = o.GenerateNames()
		if err != nil {
			return result, err
		}
		if utf8.RuneCountInString(o.Description) > 1000 {
			return result, fmt.Errorf("invalid_name")
		}
		for _, name := range names {
			result.Nodes = append(result.Nodes, LocationOperationNode{Name: name, Depth: 1, Location: true})
		}
		result.Locations = len(names)
	} else {
		if err = visit(source, 0, true); err != nil {
			return result, err
		}
	}
	if o.Action == "move" && seen[o.ParentID] {
		return result, fmt.Errorf("cycle")
	}
	conflict := func(parent uuid.UUID, name string, exclude uuid.UUID) {
		for _, e := range all {
			pid := uuid.Nil
			if e.Edges.Parent != nil {
				pid = e.Edges.Parent.ID
			}
			if e.ID != exclude && e.Edges.EntityType.IsLocation && pid == parent && strings.EqualFold(e.Name, name) {
				result.Conflicts = append(result.Conflicts, name)
				break
			}
		}
	}
	if o.Action == "generate" {
		for _, name := range names {
			conflict(sourceID, name, uuid.Nil)
		}
	} else {
		exclude := uuid.Nil
		if o.Action == "move" {
			exclude = sourceID
		}
		conflict(o.ParentID, o.Name, exclude)
	}
	if o.Action == "copy" {
		siblingNames := map[string]bool{}
		for _, e := range selected {
			if e.ID == sourceID || !e.Edges.EntityType.IsLocation {
				continue
			}
			key := e.Edges.Parent.ID.String() + "/" + strings.ToLower(e.Name)
			if siblingNames[key] {
				result.Conflicts = append(result.Conflicts, e.Name)
			}
			siblingNames[key] = true
		}
	}
	if o.Preview {
		return result, nil
	}
	if len(result.Conflicts) > 0 && !o.AllowConflicts {
		return result, fmt.Errorf("conflicts")
	}
	if o.Action == "move" {
		q := tx.Entity.UpdateOneID(sourceID).SetName(o.Name).ClearParent()
		if o.ParentID != uuid.Nil {
			q.SetParentID(o.ParentID)
		}
		if _, err = q.Save(ctx); err != nil {
			return result, err
		}
		result.RootID = sourceID
		if err = commit(); err == nil {
			r.publishMutationEvent(gid)
		}
		return result, err
	}
	next, err := r.GetHighestAssetIDTx(ctx, tx, gid)
	if err != nil {
		return result, err
	}
	createdKeys := []string{}
	var bucket *blob.Bucket
	defer func() {
		if bucket != nil {
			if err != nil {
				for _, key := range createdKeys {
					_ = bucket.Delete(context.Background(), key)
				}
			}
			_ = bucket.Close()
		}
	}()
	var cloneAttachment func(*ent.Attachment, uuid.UUID) (*ent.Attachment, error)
	cloneAttachment = func(a *ent.Attachment, owner uuid.UUID) (*ent.Attachment, error) {
		aid := uuid.New()
		path := a.Path
		if !isExternalLink(a.MimeType) {
			if bucket == nil {
				bucket, err = blob.OpenBucket(ctx, r.attachments.GetConnString())
				if err != nil {
					return nil, err
				}
			}
			path = r.attachments.path(gid, "copy-"+aid.String())
			reader, e := bucket.NewReader(ctx, r.attachments.fullPath(a.Path), nil)
			if e != nil {
				return nil, e
			}
			defer reader.Close()
			key := r.attachments.fullPath(path)
			createdKeys = append(createdKeys, key)
			writer, e := bucket.NewWriter(ctx, key, &blob.WriterOptions{ContentType: a.MimeType})
			if e != nil {
				return nil, e
			}
			_, e = io.Copy(writer, reader)
			closeErr := writer.Close()
			if e != nil {
				return nil, e
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
		q := tx.Attachment.Create().SetID(aid).SetEntityID(owner).SetType(a.Type).SetTitle(a.Title).SetMimeType(a.MimeType).SetPath(path).SetPrimary(a.Primary)
		if a.Edges.Thumbnail != nil {
			thumb, e := cloneAttachment(a.Edges.Thumbnail, owner)
			if e != nil {
				return nil, e
			}
			q.SetThumbnailID(thumb.ID)
		}
		return q.Save(ctx)
	}
	ids := map[uuid.UUID]uuid.UUID{}
	if o.Action == "generate" {
		for i, name := range names {
			id := uuid.NewSHA1(rootID, []byte(strconv.Itoa(i)))
			if i == 0 {
				id = rootID
			}
			next++
			q := tx.Entity.Create().SetID(id).SetName(name).SetDescription(o.Description).SetGroupID(gid).SetEntityTypeID(source.Edges.EntityType.ID).SetParentID(sourceID).SetAssetID(int64(next))
			if _, err = q.Save(ctx); err != nil {
				return result, err
			}
		}
	} else {
		for _, e := range selected {
			ids[e.ID] = uuid.New()
		}
		ids[sourceID] = rootID
		for _, e := range selected {
			next++
			name := e.Name
			parent := o.ParentID
			if e.ID == sourceID {
				name = o.Name
			} else if e.Edges.Parent != nil {
				parent = ids[e.Edges.Parent.ID]
			}
			q := tx.Entity.Create().SetID(ids[e.ID]).SetGroupID(gid).SetEntityTypeID(e.Edges.EntityType.ID).SetName(name).SetDescription(e.Description).SetNotes(e.Notes).SetQuantity(e.Quantity).SetAssetID(int64(next)).SetManufacturer(e.Manufacturer).SetModelNumber(e.ModelNumber).SetArchived(e.Archived)
			if parent != uuid.Nil {
				q.SetParentID(parent)
			}
			if o.SerialNumbers {
				q.SetSerialNumber(e.SerialNumber)
			}
			if o.PurchaseWarranty {
				q.SetPurchaseFrom(e.PurchaseFrom).SetLifetimeWarranty(e.LifetimeWarranty).SetWarrantyDetails(e.WarrantyDetails)
				if !e.PurchaseDate.IsZero() {
					q.SetPurchaseDate(e.PurchaseDate)
				}
				if !e.WarrantyExpires.IsZero() {
					q.SetWarrantyExpires(e.WarrantyExpires)
				}
			}
			for _, tag := range e.Edges.Tag {
				q.AddTagIDs(tag.ID)
			}
			if _, err = q.Save(ctx); err != nil {
				return result, err
			}
			for _, f := range e.Edges.Fields {
				if _, err = tx.EntityField.Create().SetEntityID(ids[e.ID]).SetName(f.Name).SetDescription(f.Description).SetTimeValue(f.TimeValue).SetType(f.Type).SetTextValue(f.TextValue).SetNumberValue(f.NumberValue).SetBooleanValue(f.BooleanValue).Save(ctx); err != nil {
					return result, err
				}
			}
			for _, a := range e.Edges.Attachments {
				if a.Type.String() == "thumbnail" {
					continue
				}
				if (a.Type.String() == "photo" && o.Photos) || (a.Type.String() != "photo" && o.Attachments) {
					if _, err = cloneAttachment(a, ids[e.ID]); err != nil {
						return result, err
					}
				}
			}
		}
	}
	result.RootID = rootID
	if err = commit(); err != nil {
		return result, err
	}
	r.publishMutationEvent(gid)
	return result, nil
}

// assertAcyclicParent also protects the regular edit and bulk patch APIs.
func (r *EntityRepository) assertAcyclicParent(ctx context.Context, gid, id, parent uuid.UUID) error {
	seen := map[uuid.UUID]bool{id: true}
	for parent != uuid.Nil {
		if seen[parent] {
			return fmt.Errorf("cycle")
		}
		seen[parent] = true
		e, err := r.db.Entity.Query().Where(entity.ID(parent), entity.HasGroupWith(group.ID(gid))).WithParent().Only(ctx)
		if err != nil {
			return err
		}
		parent = uuid.Nil
		if e.Edges.Parent != nil {
			parent = e.Edges.Parent.ID
		}
	}
	return nil
}
