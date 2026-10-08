// Package pagination centraliza el parseo de query params de paginación
// (?pagina=&tamano=) y el bloque de metadatos de las respuestas de listado.
// Criterio único del proyecto: tamaño por defecto 20, máximo 100.
//
// F03: los servicios de empleados/contratos/organización ya no tienen su
// propio PaginacionRequest; reciben un pagination.Parametros que el handler
// obtiene con pagination.Desde(c).
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	tamanoPorDefecto = 20
	tamanoMaximo     = 100

	// paginaMaxima evita desbordar int (y el int32 de LIMIT/OFFSET) con un
	// ?pagina= absurdo: 1_000_000 * 100 = 1e8 < MaxInt32.
	paginaMaxima = 1_000_000
)

// Parametros representa página/tamaño ya validados y el OFFSET calculado.
type Parametros struct {
	Pagina int
	Tamano int
	Offset int
}

// Desde lee "pagina" y "tamano" del query string, con valores por defecto
// seguros y un tope máximo para evitar listados sin límite real.
func Desde(c *gin.Context) Parametros {
	pagina, err := strconv.Atoi(c.DefaultQuery("pagina", "1"))
	if err != nil || pagina < 1 {
		pagina = 1
	}
	if pagina > paginaMaxima {
		pagina = paginaMaxima
	}

	tamano, err := strconv.Atoi(c.DefaultQuery("tamano", strconv.Itoa(tamanoPorDefecto)))
	if err != nil || tamano < 1 {
		tamano = tamanoPorDefecto
	}
	if tamano > tamanoMaximo {
		tamano = tamanoMaximo
	}

	return Parametros{
		Pagina: pagina,
		Tamano: tamano,
		Offset: (pagina - 1) * tamano,
	}
}

// Normalizado devuelve p con defaults aplicados si algún campo viene en
// cero (p. ej. un Parametros{} construido a mano en un test o job). Evita
// LIMIT 0 y la división por cero en NuevaMeta.
func (p Parametros) Normalizado() Parametros {
	if p.Pagina < 1 {
		p.Pagina = 1
	}
	if p.Pagina > paginaMaxima {
		p.Pagina = paginaMaxima
	}
	if p.Tamano < 1 {
		p.Tamano = tamanoPorDefecto
	}
	if p.Tamano > tamanoMaximo {
		p.Tamano = tamanoMaximo
	}
	p.Offset = (p.Pagina - 1) * p.Tamano
	return p
}

// Limit32 y Offset32 devuelven los valores en int32, que es lo que esperan
// las funciones de internal/db/dto (limit, offset int32).
func (p Parametros) Limit32() int32  { return int32(p.Normalizado().Tamano) }
func (p Parametros) Offset32() int32 { return int32(p.Normalizado().Offset) }

// Meta es el bloque de metadatos de paginación que se adjunta a las
// respuestas de listado, junto con el total de filas devuelto por la query
// ContarX correspondiente.
type Meta struct {
	Pagina       int   `json:"pagina"`
	Tamano       int   `json:"tamano"`
	Total        int64 `json:"total"`
	TotalPaginas int64 `json:"total_paginas"`
}

func NuevaMeta(p Parametros, total int64) Meta {
	p = p.Normalizado()
	totalPaginas := total / int64(p.Tamano)
	if total%int64(p.Tamano) != 0 {
		totalPaginas++
	}
	return Meta{
		Pagina:       p.Pagina,
		Tamano:       p.Tamano,
		Total:        total,
		TotalPaginas: totalPaginas,
	}
}