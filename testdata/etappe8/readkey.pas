program TasteUndZeile;
{ ReadKey im Zeilenmodus: liefert das erste Zeichen einer Zeile und verwirft
  den Rest samt Enter, damit ein folgendes ReadLn eine neue Zeile liest
  (Menue mit ReadKey, danach Eingaben mit ReadLn). }
uses Crt;
var
  C: Char;
  Name: string;
  Menge: Integer;
begin
  C := ReadKey;                 { Zeile "1" }
  WriteLn('Wahl: ', C);
  ReadLn(Name);                 { Zeile "Hammer" }
  ReadLn(Menge);                { Zeile "10" }
  WriteLn(Name, ' ', Menge);
  C := ReadKey;                 { Zeile "abc": nur 'a' }
  WriteLn('Taste: ', C);
  C := ReadKey;                 { leere Zeile: Enter }
  WriteLn('Enter: ', Ord(C));
  ReadLn(Name);                 { Zeile "weiter" }
  WriteLn(Name);
  C := ReadKey;                 { Dateiende }
  WriteLn('Ende: ', Ord(C))
end.
