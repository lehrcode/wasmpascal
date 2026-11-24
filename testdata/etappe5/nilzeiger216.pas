program NilZeiger;
type
  PKnoten = ^TKnoten;
  TKnoten = record
    Wert: Integer;
    Naechster: PKnoten
  end;
var
  Kopf: PKnoten;

procedure Zeige(p: PKnoten);
begin
  WriteLn(p^.Wert);
  Zeige(p^.Naechster)
end;

begin
  New(Kopf);
  Kopf^.Wert := 1;
  Kopf^.Naechster := nil;
  Zeige(Kopf)
end.
