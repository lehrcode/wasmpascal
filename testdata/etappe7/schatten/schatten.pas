program Schatten;
{ Gleiche Namen in zwei Units: die zuletzt genannte Unit gewinnt,
  qualifiziert ist jede erreichbar; eigene Deklarationen gehen vor. }
uses Erste, Zweite;
var
  Eigen: Integer;
begin
  WriteLn(Name, ' ', Erste.Name, ' ', Zweite.Name);
  WriteLn(Wert, ' ', Erste.Wert);
  Eigen := Zweite.Doppelt(21);
  WriteLn(Eigen, ' ', Doppelt(5), ' ', Erste.Doppelt(5));
  WriteLn(System.Length(Name))
end.
