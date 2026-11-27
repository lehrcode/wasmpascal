program Signatur;
type
  TFunc = function(X: Integer): Integer;

function Halb(X: Real): Real;
begin
  Halb := X / 2
end;

var
  F: TFunc;
begin
  F := Halb
end.
