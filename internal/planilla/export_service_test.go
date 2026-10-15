package planilla

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func filaOK() FilaDispersionBancaria {
	return FilaDispersionBancaria{
		EmpleadoID: 7, Cedula: "1-0234-0567", NombreCompleto: "Ana Mora",
		IBAN: "CR05015202001026284066", NetoPagado: dec("150000.00"),
	}

}

func TestValidarFilaDispersion(t *testing.T) {
	casos := []struct {
		nombre string
		mod    func(*FilaDispersionBancaria)
		motivo string // "" = válida
	}{
		{"fila válida", func(f *FilaDispersionBancaria) {}, ""},
		{"IBAN vacío", func(f *FilaDispersionBancaria) { f.IBAN = "" }, MotivoIBANVacio},
		{"IBAN solo espacios", func(f *FilaDispersionBancaria) { f.IBAN = "   \t " }, MotivoIBANVacio},
		{"IBAN con coma", func(f *FilaDispersionBancaria) { f.IBAN = "CR05,0152" }, MotivoIBANInvalido},
		{"IBAN con salto de línea", func(f *FilaDispersionBancaria) { f.IBAN = "CR05\n0152" }, MotivoIBANInvalido},
		{"IBAN con NUL", func(f *FilaDispersionBancaria) { f.IBAN = "CR05\x000152" }, MotivoIBANInvalido},
		{"cédula vacía", func(f *FilaDispersionBancaria) { f.Cedula = "" }, MotivoCedulaVacia},
		{"cédula con CR", func(f *FilaDispersionBancaria) { f.Cedula = "1-0234\r0567" }, MotivoCedulaInvalida},
		{"cédula con coma", func(f *FilaDispersionBancaria) { f.Cedula = "1,0234" }, MotivoCedulaInvalida},
		{"neto cero", func(f *FilaDispersionBancaria) { f.NetoPagado = decimal.Zero }, MotivoNetoNoPositivo},
		{"neto negativo", func(f *FilaDispersionBancaria) { f.NetoPagado = dec("-0.01") }, MotivoNetoNoPositivo},
		{"neto mínimo 0.01 válido", func(f *FilaDispersionBancaria) { f.NetoPagado = dec("0.01") }, ""},
		{"neto con 3 decimales", func(f *FilaDispersionBancaria) { f.NetoPagado = dec("100.005") }, MotivoNetoDecimales},
		{"neto 2 decimales con ceros extra válido", func(f *FilaDispersionBancaria) { f.NetoPagado = dec("100.500") }, ""},
		{"neto 1 decimal válido", func(f *FilaDispersionBancaria) { f.NetoPagado = dec("100.5") }, ""},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			f := filaOK()
			tc.mod(&f)
			err := validarFilaDispersion(f)
			if tc.motivo == "" {
				if err != nil {
					t.Fatalf("no esperaba error, obtuve %v", err)
				}
				return
			}
			var fe *FilaDispersionError
			if !errors.As(err, &fe) {
				t.Fatalf("esperaba *FilaDispersionError, obtuve %v", err)
			}
			if fe.Motivo != tc.motivo || fe.EmpleadoID != f.EmpleadoID {
				t.Fatalf("motivo/empleado = %q/%d, quería %q/%d", fe.Motivo, fe.EmpleadoID, tc.motivo, f.EmpleadoID)
			}
		})
	}
}

func TestObtenerBoletaPagoAutorizada_AplicaPermisoPropioEnServicio(t *testing.T) {
	service, _ := nuevoSvc(&fakeTx{})
	_, err := service.ObtenerBoletaPagoAutorizada(context.Background(), 4, 9, 7, []string{"planilla.ver_boleta_propia"})
	if !errors.Is(err, ErrBoletaNoPropia) {
		t.Fatalf("acceso a la boleta de otro empleado: error = %v", err)
	}
	if _, err := service.ObtenerBoletaPagoAutorizada(context.Background(), 4, 7, 7, []string{"planilla.ver_boleta_propia"}); err != nil {
		t.Fatalf("acceso a la boleta propia: %v", err)
	}
	if _, err := service.ObtenerBoletaPagoAutorizada(context.Background(), 4, 9, 7, []string{"planilla.ver_boleta"}); err != nil {
		t.Fatalf("permiso global de boletas no debe filtrar por empleado: %v", err)
	}
	if _, err := service.ObtenerBoletaPagoAutorizada(context.Background(), 4, 7, 7, nil); !errors.Is(err, ErrBoletaSinPermiso) {
		t.Fatalf("sin permiso, error = %v", err)
	}
}

func TestSanitizarCampoDispersion(t *testing.T) {
	casos := []struct{ nombre, in, out string }{
		{"vacío", "", ""},
		{"solo espacios", "   ", ""},
		{"sin cambios", "María José Pérez", "María José Pérez"},
		{"coma", "Pérez, Ana", "Pérez  Ana"},
		{"CRLF interno", "Ana\r\nMora", "Ana  Mora"},
		{"LF al final", "Ana Mora\n", "Ana Mora"},
		{"tab", "Ana\tMora", "Ana Mora"},
		{"NUL", "Ana\x00Mora", "Ana Mora"},
		{"ESC y DEL", "Ana\x1b[0mMora\x7f", "Ana [0mMora"},
		{"C1 control (U+0085 NEL)", "Ana\u0085Mora", "Ana Mora"},
		{"solo caracteres de control", "\r\n\t\x00", ""},
		{"comillas se conservan", `Ana "Mora"`, `Ana "Mora"`},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			got := sanitizarCampoDispersion(tc.in)
			if got != tc.out {
				t.Fatalf("sanitizar(%q) = %q, quería %q", tc.in, got, tc.out)
			}
			if strings.ContainsAny(got, ",\r\n\x00") {
				t.Fatalf("resultado %q aún contiene separador o control", got)
			}
		})
	}
}

func TestArmarArchivoDispersion(t *testing.T) {
	t.Run("formato, CRLF y total", func(t *testing.T) {
		filas := []FilaDispersionBancaria{
			{1, "1-1", "Ana, Mora", " CR01 ", dec("1000.5")},
			{2, "2-2", "Luis", "CR02", dec("0.25")},
		}
		buf, total, err := armarArchivoDispersion(filas)
		if err != nil {
			t.Fatal(err)
		}
		want := "CR01,1-1,Ana  Mora,1000.50\r\nCR02,2-2,Luis,0.25\r\n"
		if string(buf) != want {
			t.Fatalf("contenido = %q, quería %q", buf, want)
		}
		if !total.Equal(dec("1000.75")) {
			t.Fatalf("total = %s", total)
		}
	})
	t.Run("una fila inválida aborta todo", func(t *testing.T) {
		filas := []FilaDispersionBancaria{filaOK(), {EmpleadoID: 9, Cedula: "x", IBAN: "", NetoPagado: dec("1")}}
		buf, _, err := armarArchivoDispersion(filas)
		if err == nil || buf != nil {
			t.Fatalf("esperaba error y sin buffer, obtuve %v / %q", err, buf)
		}
	})
}

// ---- fakes para el servicio ----

type fakeTx struct {
	filas     []FilaDispersionBancaria
	registros []ExportacionRegistro
	errLeer   error
	errReg    error
}

func (f *fakeTx) ObtenerDetallesPlanillaParaDispersion(context.Context, int64) ([]FilaDispersionBancaria, error) {
	return f.filas, f.errLeer
}
func (f *fakeTx) ObtenerDatosBoletaPago(context.Context, int64, int64) (EncabezadoBoletaPago, error) {
	return EncabezadoBoletaPago{}, nil
}
func (f *fakeTx) ObtenerDesgloseConceptosBoletaPago(context.Context, int64, int64) ([]ConceptoBoletaPago, error) {
	return nil, nil
}
func (f *fakeTx) ObtenerEmbargosBoletaPago(context.Context, int64, int64) ([]EmbargoBoletaPago, error) {
	return nil, nil
}
func (f *fakeTx) RegistrarExportacion(_ context.Context, r ExportacionRegistro) (int64, error) {
	if f.errReg != nil {
		return 0, f.errReg
	}
	f.registros = append(f.registros, r)
	return int64(len(f.registros)), nil
}

// fakeUoW simula rollback: si fn falla, descarta los registros.
type fakeUoW struct {
	tx        *fakeTx
	usuarioID int64
	llamadas  int
}

func (u *fakeUoW) EjecutarAuditado(ctx context.Context, usuarioID int64, fn func(context.Context, ExportTx) error) error {
	u.llamadas++
	u.usuarioID = usuarioID
	antes := len(u.tx.registros)
	if err := fn(ctx, u.tx); err != nil {
		u.tx.registros = u.tx.registros[:antes]
		return err
	}
	return nil
}

func nuevoSvc(tx *fakeTx) (*ExportService, *fakeUoW) {
	u := &fakeUoW{tx: tx}
	s := NewExportService(tx, u)
	s.now = func() time.Time { return time.Date(2026, 10, 13, 8, 30, 0, 0, time.UTC) }
	return s, u
}

func TestGenerarArchivoDispersionBancaria(t *testing.T) {
	t.Run("registra hash, filas, total y usuario", func(t *testing.T) {
		tx := &fakeTx{filas: []FilaDispersionBancaria{filaOK(), {2, "2-2", "Luis", "CR02", dec("2000.25")}}}
		s, u := nuevoSvc(tx)
		buf, nombre, err := s.GenerarArchivoDispersionBancaria(context.Background(), 42, 5)
		if err != nil {
			t.Fatal(err)
		}

		if nombre != "dispersion_bancaria_periodo_42_20261013_083000.txt" {
			t.Fatalf("nombre = %s", nombre)
		}
		if len(tx.registros) != 1 || u.usuarioID != 5 {
			t.Fatalf("registros=%d usuario=%d", len(tx.registros), u.usuarioID)
		}
		r := tx.registros[0]
		h := sha256.Sum256(buf)
		if r.HashSHA256 != hex.EncodeToString(h[:]) || len(r.HashSHA256) != 64 {
			t.Fatalf("hash no coincide con el contenido entregado: %s", r.HashSHA256)
		}
		if r.CantidadFilas != 2 || !r.TotalMonto.Equal(dec("152000.25")) || r.PeriodoID != 42 ||
			r.UsuarioID != 5 || r.NombreArchivo != nombre || r.FormatoCodigo != FormatoDispersionTXT {
			t.Fatalf("registro incorrecto: %+v", r)
		}
	})

	casos := []struct {
		nombre    string
		tx        *fakeTx
		usuarioID int64
		check     func(error) bool
		llamadas  int
	}{
		{"usuario 0", &fakeTx{filas: []FilaDispersionBancaria{filaOK()}}, 0,
			func(e error) bool { return errors.Is(e, ErrUsuarioExportRequerido) }, 0},
		{"sin filas", &fakeTx{}, 5,
			func(e error) bool { return errors.Is(e, ErrPlanillaSinFilasParaDispersion) }, 1},
		{"fila inválida", &fakeTx{filas: []FilaDispersionBancaria{{EmpleadoID: 3, Cedula: "1", IBAN: "", NetoPagado: dec("1")}}}, 5,
			func(e error) bool { var fe *FilaDispersionError; return errors.As(e, &fe) && fe.EmpleadoID == 3 }, 1},
		{"falla lectura", &fakeTx{errLeer: errors.New("boom")}, 5,
			func(e error) bool { return e != nil }, 1},
		{"falla registro", &fakeTx{filas: []FilaDispersionBancaria{filaOK()}, errReg: errors.New("boom")}, 5,
			func(e error) bool { return e != nil }, 1},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			s, u := nuevoSvc(tc.tx)
			buf, nombre, err := s.GenerarArchivoDispersionBancaria(context.Background(), 1, tc.usuarioID)
			if !tc.check(err) {
				t.Fatalf("error inesperado: %v", err)
			}
			if buf != nil || nombre != "" {
				t.Fatalf("no debe entregarse archivo en caso de error")
			}
			if len(tc.tx.registros) != 0 {
				t.Fatalf("no debe quedar registro en caso de error")
			}
			if u.llamadas != tc.llamadas {
				t.Fatalf("llamadas a la UoW = %d, quería %d", u.llamadas, tc.llamadas)
			}
		})
	}
}
