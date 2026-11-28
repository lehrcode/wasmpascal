program Adressiert;
{ Die Routine Plus ist nur ueber eine typisierte Konstante und @ erreichbar;
  sie ist damit Wurzel der Erreichbarkeit und muss im Modul landen (Etappe 18). }
type
  TBinOp = function(A, B: Integer): Integer;
function Plus(A, B: Integer): Integer;
begin
  Plus := A + B
end;
const
  KOps: array[1..1] of TBinOp = (Plus);
var
  F: TBinOp;
begin
  F := KOps[1];
  WriteLn(F(3, 4));
  F := TBinOp(@Plus);
  WriteLn(F(5, 6))
end.
