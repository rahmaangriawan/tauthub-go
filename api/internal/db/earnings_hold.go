package db

import "fmt"

// PartnerEarningHoldHours: masa tunggu pendapatan mitra sebelum bisa ditarik
// (H+2 setelah terbit). Pendapatan baru ditulis berstatus 'pending'; setelah
// created_at lewat masa ini dan belum masuk pengajuan, statusnya dirilis
// menjadi 'settled' (saldo yang bisa ditarik).
const PartnerEarningHoldHours = 48

// PartnerEarningHoldCutoffSQL: ekspresi SQL batas waktu masa tunggu. Pendapatan
// dengan created_at <= nilai ini sudah lewat masa tunggu.
var PartnerEarningHoldCutoffSQL = fmt.Sprintf("NOW() - INTERVAL '%d hours'", PartnerEarningHoldHours)
