package huntgroup

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// SIPDialog represents a SIP dialog state
type SIPDialog struct {
	DialogID      string                  `json:"dialog_id"`
	CallID        string                  `json:"call_id"`
	LocalTag      string                  `json:"local_tag"`
	RemoteTag     string                  `json:"remote_tag"`
	LocalURI      string                  `json:"local_uri"`
	RemoteURI     string                  `json:"remote_uri"`
	RemoteTarget  string                  `json:"remote_target"`  // Contact URI from remote party
	RouteSet      []string                `json:"route_set"`      // Record-Route headers
	LocalCSeq     uint32                  `json:"local_cseq"`     // Local CSeq number
	RemoteCSeq    uint32                  `json:"remote_cseq"`    // Remote CSeq number
	State         DialogState             `json:"state"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	mutex         sync.RWMutex            `json:"-"`
}

// DialogState represents the state of a SIP dialog
type DialogState string

const (
	DialogStateEarly       DialogState = "early"       // Dialog created but not confirmed
	DialogStateConfirmed   DialogState = "confirmed"   // Dialog confirmed with 2xx response
	DialogStateTerminated  DialogState = "terminated"  // Dialog terminated
)

// TransactionCorrelation manages the correlation between A-leg and B-leg transactions
type TransactionCorrelation struct {
	AlegTransactionID string                     `json:"aleg_transaction_id"`
	BlegTransactionID string                     `json:"bleg_transaction_id"`
	Method           string                     `json:"method"`
	State            CorrelationState           `json:"state"`
	AlegTransaction  transaction.Transaction    `json:"-"`
	BlegTransaction  transaction.Transaction    `json:"-"`
	CreatedAt        time.Time                  `json:"created_at"`
	mutex            sync.RWMutex               `json:"-"`
}

// CorrelationState represents the state of transaction correlation
type CorrelationState string

const (
	CorrelationStateActive     CorrelationState = "active"
	CorrelationStateCompleted  CorrelationState = "completed"
	CorrelationStateTerminated CorrelationState = "terminated"
)

// DialogManager manages SIP dialogs for B2BUA sessions
type DialogManager struct {
	dialogs       map[string]*SIPDialog           // dialogID -> dialog
	dialogsByCall map[string][]*SIPDialog         // callID -> dialogs
	correlations  map[string]*TransactionCorrelation // correlationID -> correlation
	mutex         sync.RWMutex
	logger        logging.Logger
}

// NewDialogManager creates a new dialog manager
func NewDialogManager(logger logging.Logger) *DialogManager {
	return &DialogManager{
		dialogs:       make(map[string]*SIPDialog),
		dialogsByCall: make(map[string][]*SIPDialog),
		correlations:  make(map[string]*TransactionCorrelation),
		logger:        logger,
	}
}

// CreateDialog creates a new SIP dialog
func (dm *DialogManager) CreateDialog(callID, localURI, remoteURI, localTag, remoteTag string) *SIPDialog {
	dialogID := dm.generateDialogID(callID, localTag, remoteTag)
	now := time.Now().UTC()

	dialog := &SIPDialog{
		DialogID:     dialogID,
		CallID:       callID,
		LocalTag:     localTag,
		RemoteTag:    remoteTag,
		LocalURI:     localURI,
		RemoteURI:    remoteURI,
		LocalCSeq:    1,
		RemoteCSeq:   0,
		State:        DialogStateEarly,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	dm.mutex.Lock()
	dm.dialogs[dialogID] = dialog
	dm.dialogsByCall[callID] = append(dm.dialogsByCall[callID], dialog)
	dm.mutex.Unlock()

	dm.logger.Debug("Created SIP dialog",
		logging.Field{Key: "dialog_id", Value: dialogID},
		logging.Field{Key: "call_id", Value: callID},
		logging.Field{Key: "local_tag", Value: localTag},
		logging.Field{Key: "remote_tag", Value: remoteTag})

	return dialog
}

// GetDialog retrieves a dialog by ID
func (dm *DialogManager) GetDialog(dialogID string) *SIPDialog {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	return dm.dialogs[dialogID]
}

// FindDialog finds a dialog by Call-ID and tags
func (dm *DialogManager) FindDialog(callID, localTag, remoteTag string) *SIPDialog {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	dialogs := dm.dialogsByCall[callID]
	for _, dialog := range dialogs {
		dialog.RLock()
		match := (dialog.LocalTag == localTag && dialog.RemoteTag == remoteTag) ||
			(dialog.LocalTag == remoteTag && dialog.RemoteTag == localTag)
		dialog.RUnlock()
		
		if match {
			return dialog
		}
	}
	return nil
}

// UpdateDialog updates dialog state
func (dm *DialogManager) UpdateDialog(dialog *SIPDialog) {
	dialog.Lock()
	dialog.UpdatedAt = time.Now().UTC()
	dialog.Unlock()

	dm.mutex.Lock()
	dm.dialogs[dialog.DialogID] = dialog
	dm.mutex.Unlock()
}

// TerminateDialog terminates a dialog
func (dm *DialogManager) TerminateDialog(dialogID string) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dialog := dm.dialogs[dialogID]
	if dialog != nil {
		dialog.Lock()
		dialog.State = DialogStateTerminated
		dialog.UpdatedAt = time.Now().UTC()
		dialog.Unlock()

		dm.logger.Debug("Terminated SIP dialog",
			logging.Field{Key: "dialog_id", Value: dialogID})
	}
}

// CreateCorrelation creates a transaction correlation
func (dm *DialogManager) CreateCorrelation(alegTxnID, blegTxnID, method string) *TransactionCorrelation {
	correlationID := fmt.Sprintf("%s-%s", alegTxnID, blegTxnID)
	now := time.Now().UTC()

	correlation := &TransactionCorrelation{
		AlegTransactionID: alegTxnID,
		BlegTransactionID: blegTxnID,
		Method:           method,
		State:            CorrelationStateActive,
		CreatedAt:        now,
	}

	dm.mutex.Lock()
	dm.correlations[correlationID] = correlation
	dm.mutex.Unlock()

	dm.logger.Debug("Created transaction correlation",
		logging.Field{Key: "correlation_id", Value: correlationID},
		logging.Field{Key: "aleg_txn", Value: alegTxnID},
		logging.Field{Key: "bleg_txn", Value: blegTxnID},
		logging.Field{Key: "method", Value: method})

	return correlation
}

// FindCorrelationByAleg finds correlation by A-leg transaction ID
func (dm *DialogManager) FindCorrelationByAleg(alegTxnID string) *TransactionCorrelation {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	for _, correlation := range dm.correlations {
		if correlation.AlegTransactionID == alegTxnID {
			return correlation
		}
	}
	return nil
}

// FindCorrelationByBleg finds correlation by B-leg transaction ID
func (dm *DialogManager) FindCorrelationByBleg(blegTxnID string) *TransactionCorrelation {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	for _, correlation := range dm.correlations {
		if correlation.BlegTransactionID == blegTxnID {
			return correlation
		}
	}
	return nil
}

// Dialog management methods
func (d *SIPDialog) Lock() {
	d.mutex.Lock()
}

func (d *SIPDialog) Unlock() {
	d.mutex.Unlock()
}

func (d *SIPDialog) RLock() {
	d.mutex.RLock()
}

func (d *SIPDialog) RUnlock() {
	d.mutex.RUnlock()
}

func (d *SIPDialog) GetNextLocalCSeq() uint32 {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.LocalCSeq++
	return d.LocalCSeq
}

func (d *SIPDialog) UpdateRemoteCSeq(cseq uint32) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if cseq > d.RemoteCSeq {
		d.RemoteCSeq = cseq
	}
}

func (d *SIPDialog) SetRemoteTarget(contact string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.RemoteTarget = contact
}

func (d *SIPDialog) SetRouteSet(routes []string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.RouteSet = make([]string, len(routes))
	copy(d.RouteSet, routes)
}

func (d *SIPDialog) ConfirmDialog() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.State = DialogStateConfirmed
	d.UpdatedAt = time.Now().UTC()
}

func (d *SIPDialog) IsConfirmed() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.State == DialogStateConfirmed
}

func (d *SIPDialog) IsTerminated() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.State == DialogStateTerminated
}

// Transaction correlation methods
func (tc *TransactionCorrelation) Lock() {
	tc.mutex.Lock()
}

func (tc *TransactionCorrelation) Unlock() {
	tc.mutex.Unlock()
}

func (tc *TransactionCorrelation) RLock() {
	tc.mutex.RLock()
}

func (tc *TransactionCorrelation) RUnlock() {
	tc.mutex.RUnlock()
}

func (tc *TransactionCorrelation) SetAlegTransaction(txn transaction.Transaction) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.AlegTransaction = txn
}

func (tc *TransactionCorrelation) SetBlegTransaction(txn transaction.Transaction) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.BlegTransaction = txn
}

func (tc *TransactionCorrelation) Complete() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.State = CorrelationStateCompleted
}

func (tc *TransactionCorrelation) Terminate() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.State = CorrelationStateTerminated
}

func (tc *TransactionCorrelation) IsActive() bool {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return tc.State == CorrelationStateActive
}

// Helper methods
func (dm *DialogManager) generateDialogID(callID, localTag, remoteTag string) string {
	return fmt.Sprintf("%s-%s-%s", callID, localTag, remoteTag)
}

// ExtractTagFromHeader extracts tag parameter from From/To header
func ExtractTagFromHeader(headerValue string) string {
	if tagStart := strings.Index(headerValue, "tag="); tagStart != -1 {
		tagStart += 4
		tagEnd := strings.Index(headerValue[tagStart:], ";")
		if tagEnd == -1 {
			return headerValue[tagStart:]
		}
		return headerValue[tagStart : tagStart+tagEnd]
	}
	return ""
}

// ExtractURIFromHeader extracts URI from From/To header
func ExtractURIFromHeader(headerValue string) string {
	// Handle both "Display Name" <sip:uri> and <sip:uri> formats
	if start := strings.Index(headerValue, "<"); start != -1 {
		if end := strings.Index(headerValue[start:], ">"); end != -1 {
			return headerValue[start+1 : start+end]
		}
	}
	
	// Handle bare URI format
	parts := strings.Fields(headerValue)
	if len(parts) > 0 {
		uri := parts[0]
		if semicolon := strings.Index(uri, ";"); semicolon != -1 {
			uri = uri[:semicolon]
		}
		return uri
	}
	
	return headerValue
}

// BuildHeaderWithTag builds a From/To header with tag parameter
func BuildHeaderWithTag(uri, displayName, tag string) string {
	var header string
	
	if displayName != "" {
		header = fmt.Sprintf("\"%s\" <%s>", displayName, uri)
	} else {
		header = fmt.Sprintf("<%s>", uri)
	}
	
	if tag != "" {
		header += fmt.Sprintf(";tag=%s", tag)
	}
	
	return header
}

// ExtractCSeqNumber extracts CSeq number from CSeq header
func ExtractCSeqNumber(cseqHeader string) uint32 {
	parts := strings.Fields(cseqHeader)
	if len(parts) > 0 {
		if cseq, err := parseUint32(parts[0]); err == nil {
			return cseq
		}
	}
	return 0
}

// ExtractCSeqMethod extracts method from CSeq header
func ExtractCSeqMethod(cseqHeader string) string {
	parts := strings.Fields(cseqHeader)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// parseUint32 is a helper function to parse uint32
func parseUint32(s string) (uint32, error) {
	var result uint32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid character: %c", c)
		}
		result = result*10 + uint32(c-'0')
	}
	return result, nil
}