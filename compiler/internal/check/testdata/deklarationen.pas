program Deklarationen;
const
  Zehn = 10;
  Hex = $FF;
  Negativ = -Zehn;
  Halb = 0.5;
  Zeichen = 'z';
  Text_ = 'Hallo';
  Leer = '';
  Wahr = not False;
  Menge = [1, 3..5];
  Bereich = Zehn * 2 + Hex div 16;
  Verkettet = 'ab' + 'c' + #33;
  Laenge = Length(Verkettet);
  Kode = Ord('A') + Ord(Wahr);
  Gross = High(LongInt);
  Klein = Low(ShortInt);
type
  TFarbe = (Rot, Gruen, Blau);
  THell = Gruen..Blau;
  TZiffer = 0..9;
  TBuchstabe = 'a'..'z';
  TGross = 1000..1100;
  TNeg = -100..100;
  TName = string[20];
  TFeld = array[1..10] of Integer;
  TMatrix = array[1..3, 1..4] of Real;
  TBits = packed array[0..7] of Boolean;
  TZaehler = array[Char] of Word;
  TNachFarbe = array[TFarbe] of TName;
  TMischung = record
    b: Byte;
    i: Integer;
    c: Char;
    r: Real;
    s: TName;
    w: Word
  end;
  TForm = (Kreis, Rechteck);
  TFigur = record
    x, y: Integer;
    case Art: TForm of
      Kreis: (Radius: Real);
      Rechteck: (Breite, Hoehe: Integer; Gefuellt: Boolean)
  end;
  TUnion = record
    case Integer of
      0: (L: LongInt);
      1: (B0, B1, B2, B3: Byte)
  end;
  TZiffern = set of TZiffer;
  TZeichen = set of Char;
  TFarben = set of TFarbe;
  PKnoten = ^TKnoten;
  TKnoten = record
    Wert: Integer;
    Naechster: PKnoten
  end;
  PString = ^string;
  TDatei = file of TMischung;
  TRoh = file;
  TFeldAlias = TFeld;
var
  i: Integer;
  f: TFarbe;
  m: TMischung;
  t: Text;
  p: Pointer;
  a, b: array[1..3] of Char;
const
  Tage: array[1..3] of Byte = (31, 28, 31);
  Ursprung: TFigur = (x: 0; y: 0; Art: Kreis; Radius: 1.5);
  Kennung: packed array[1..4] of Char = 'WASM';
  Erlaubt: TZiffern = [1, 2, 3];
  Nichts: PKnoten = nil;
  Einheit: TMatrix = ((1, 0, 0, 0), (0, 1, 0, 0), (0, 0, 1, 0));
  Name: TName = 'Pascal';
  Faktor: Real = 3;
  Einzeln: array[1..1] of Integer = (5);
begin
end.
