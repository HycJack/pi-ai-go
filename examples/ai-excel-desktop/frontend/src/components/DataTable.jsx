// AG-Grid 数据表格封装
import { useEffect, useRef } from "react";
import { Grid } from "ag-grid-community";
import "ag-grid-community/styles/ag-grid.css";
import "ag-grid-community/styles/ag-theme-alpine.css";

export function DataTable({ headers, rows, types, onReady }) {
  const containerRef = useRef(null);
  const gridInstanceRef = useRef(null);

  useEffect(() => {
    if (!containerRef.current || !headers.length) return;

    // ★ 销毁旧实例，避免内存泄漏
    if (gridInstanceRef.current) {
      gridInstanceRef.current.destroy();
      gridInstanceRef.current = null;
    }

    const columnDefs = headers.map((header) => ({
      field: header,
      headerName: header,
      sortable: true,
      filter: true,
      resizable: true,
      minWidth: 100,
      flex: 1,
      valueFormatter: (p) => {
        if (p.value === undefined || p.value === null || p.value === "")
          return "";
        if (typeof p.value === "number") return p.value.toLocaleString();
        return String(p.value);
      },
      cellClass: () =>
        types?.[header] === "numeric" ? "text-right font-mono" : "text-left",
    }));

    const gridOptions = {
      columnDefs,
      rowData: rows,
      rowBuffer: 10,
      rowHeight: 36,
      headerHeight: 42,
      animateRows: false,
      suppressColumnVirtualisation: false,
      suppressRowVirtualisation: false,
      enableCellTextSelection: true,
      onGridReady: (p) => {
        onReady?.(p.api);
      },
    };

    gridInstanceRef.current = new Grid(containerRef.current, gridOptions);

    return () => {
      if (gridInstanceRef.current) {
        gridInstanceRef.current.destroy();
        gridInstanceRef.current = null;
      }
    };
  }, [headers, rows, types, onReady]);

  return (
    <div
      ref={containerRef}
      className="ag-theme-alpine h-full w-full"
    />
  );
}
