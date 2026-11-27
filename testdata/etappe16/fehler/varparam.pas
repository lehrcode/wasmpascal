program VarParam;
type
  TProc = procedure(var X: Integer);

procedure Wert(X: Integer);
begin
end;

var
  P: TProc;
begin
  P := Wert
end.
