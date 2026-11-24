program Trap;
{ Ein Zugriff weit hinter das Speicherende ist ein echter WASM-Trap; die
  Meldung kommt ueber die vorgemerkte Quelltextstelle. }
type
  PInteger = ^Integer;
var
  p: PInteger;

procedure Schreibe(Adresse: LongInt);
begin
  p := Ptr(0, Adresse);
  WriteLn('schreibe');
  p^ := 1;
  WriteLn('nie')
end;

begin
  Schreibe($7FFFFFF0)
end.
