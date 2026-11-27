program Verschachtelt;
type
  TProc = procedure;
var
  P: TProc;

procedure Aussen;
  procedure Innen;
  begin
  end;
begin
  P := Innen
end;

begin
  Aussen
end.
