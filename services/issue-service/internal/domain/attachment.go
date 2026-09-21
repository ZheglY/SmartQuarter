package domain

import "time"

type AttachmentStatus string

const (
	Uploading AttachmentStatus = "UPLOADING"
	Ready     AttachmentStatus = "READY"
	Attached  AttachmentStatus = "ATTACHED"
	Rejected  AttachmentStatus = "REJECTED"
	Expired   AttachmentStatus = "EXPIRED"
)

type Attachment struct {
	ID, HouseID, IssueID, UploadedBy, ObjectKey, OriginalFilename, MIMEType string
	SizeBytes                                                               int64
	SHA256, ETag                                                            string
	Status                                                                  AttachmentStatus
	UploadExpiresAt, CreatedAt, UpdatedAt                                   time.Time
}

func (a *Attachment) Transition(target AttachmentStatus) error {
	if (a.Status == Uploading && (target == Ready || target == Rejected || target == Expired)) || (a.Status == Ready && target == Attached) {
		a.Status = target
		return nil
	}
	return Fail(ErrPrecondition, "attachment transition not allowed")
}
func (a Attachment) OwnedBy(actor Actor) error {
	if err := actor.InHouse(a.HouseID); err != nil {
		return err
	}
	if a.UploadedBy != actor.UserID {
		return ErrPermission
	}
	return nil
}
